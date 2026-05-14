package main

import (
	"bufio"
	"bytes"
	crand "crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/light2000/laravel-modeler-studio/ai"
	"github.com/light2000/laravel-modeler-studio/conf"
	"github.com/light2000/laravel-modeler-studio/logger"
	protopkg "github.com/light2000/laravel-modeler-studio/proto"
	"github.com/light2000/laravel-modeler-studio/trans"
)

//go:embed dist
var distEmbed embed.FS

var (
	configPath        string
	latestVersionFile string
)

type generateJob struct {
	ch chan string
}

var (
	jobsMu sync.Mutex
	jobs   = make(map[string]*generateJob)
)

func init() {
	flag.StringVar(&configPath, "config", "", "配置文件路径（必填）")
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	//isnotexist来判断，是不是不存在的错误
	if os.IsNotExist(err) { //如果返回的错误类型使用os.isNotExist()判断为true，说明文件或者文件夹不存在
		return false
	}

	return false
}

func initLogger() error {
	logPath := filepath.Join(conf.Config.LogPath, "studio.log")
	if err := logger.Init(logPath, logger.InfoLevel); err != nil {
		return fmt.Errorf("init logger failed: %w", err)
	}
	return nil
}

func main() {
	flag.Parse()
	if configPath == "" {
		fmt.Fprintf(os.Stderr, "错误: 必须指定 -config 配置文件路径\n")
		flag.Usage()
		os.Exit(1)
	}

	if err := conf.LoadConfig(configPath); err != nil {
		log.Fatalf("读取JSON配置失败: %v", err)
	}

	conf.InitFeatureStatus()
	ai.Init()
	trans.Init()

	latestVersionFile = filepath.Join(conf.Config.DataPath, "latest.json")
	if !PathExists(latestVersionFile) {
		log.Fatalf("latest version file not found: %s", latestVersionFile)
	}
	if err := initLogger(); err != nil {
		log.Fatalf("init logger failed: %v", err)
	}

	ai.TplInit(conf.Config.PromptPath)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/feature-status", handleGetFeatureStatus)
	mux.HandleFunc("GET /api/schema", handleGetSchema)
	mux.HandleFunc("PUT /api/schema", handlePutSchema)
	mux.HandleFunc("POST /api/generate", handleGenerate)
	mux.HandleFunc("GET /api/generate/{id}/stream", handleGenerateStream)
	mux.HandleFunc("GET /api/translate", handleTranslate)
	mux.HandleFunc("POST /api/ai/suggest-attrs", handleSuggestAttrsAI)

	distFS, err := fs.Sub(distEmbed, "dist")
	if err != nil {
		log.Fatalf("embed dist: %v", err)
	}
	mux.Handle("/", spaStaticHandler(distFS))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})

	logger.Infof("服务启动: config=%s\n", configPath)
	port := "0"
	if conf.Config.StudioServerPort == "auto" {
		port = "0"
	} else {
		port = conf.Config.StudioServerPort
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		logger.Fatalf("无法解析端口号: %v", err)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", portInt))
	if err != nil {
		logger.Fatalf("监听失败: %v", err)
	}
	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		logger.Fatalf("无法解析监听地址: %v", ln.Addr())
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", tcpAddr.Port)
	logger.Infof("Laravel Modeler Studio HTTP 服务已监听: %s\n", baseURL)

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- http.Serve(ln, handler)
	}()

	if waitHTTPServerReady(baseURL, 10*time.Second) {
		if conf.Config.StudioAutoOpen {
			logger.Infof("Laravel Modeler Studio HTTP 服务已就绪，正在打开浏览器: %s\n", baseURL)
			if err := openBrowser(baseURL); err != nil {
				logger.Infof("Laravel Modeler Studio 打开浏览器失败: %v\n", err)
			}
		} else {
			logger.Infof("Laravel Modeler Studio HTTP 服务已就绪，但未配置自动打开浏览器，请手动在浏览器访问: %s\n", baseURL)
		}
	} else {
		logger.Infof("等待 Laravel Modeler Studio HTTP 服务就绪超时，请手动在浏览器访问: %s\n", baseURL)
	}

	if err := <-serveErr; err != nil {
		logger.Fatalf("服务运行失败: %v", err)
	}
}

// waitHTTPServerReady 轮询根路径直到可响应或超时。
func waitHTTPServerReady(baseURL string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(baseURL + "/")
		if err == nil {
			_ = resp.Body.Close()
			return true
		}
		time.Sleep(30 * time.Millisecond)
	}
	return false
}

// openBrowser 使用系统默认浏览器打开 URL（Windows / macOS / Linux）。
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// spaStaticHandler 提供 dist 静态资源；对 History 模式下不存在的路径回退到 index.html。
func spaStaticHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		urlPath := path.Clean(r.URL.Path)
		if !strings.HasPrefix(urlPath, "/") {
			urlPath = "/" + urlPath
		}
		rel := strings.TrimPrefix(urlPath, "/")

		// 注意：勿将 Path 设为 /index.html。net/http.FileServer 会对以 /index.html 结尾的 URL
		// 发 301 到 ./（即 /），与根路径再改回 index 组合会形成无限重定向。
		serve := r
		if rel == "" {
			r2 := *r
			u2 := *r.URL
			r2.URL = &u2
			u2.Path = "/"
			u2.RawPath = ""
			serve = &r2
		} else if !fs.ValidPath(rel) {
			http.NotFound(w, r)
			return
		} else if _, statErr := fs.Stat(fsys, rel); statErr != nil {
			r2 := *r
			u2 := *r.URL
			r2.URL = &u2
			u2.Path = "/"
			u2.RawPath = ""
			serve = &r2
		}

		fileServer.ServeHTTP(w, serve)
	})
}

func handleGetFeatureStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(conf.Feature)
}

func getProject() (*protopkg.Project, error) {
	data, err := os.ReadFile(latestVersionFile)
	if err != nil {
		return nil, err
	}
	var proj protopkg.Project
	if err := protojson.Unmarshal(data, &proj); err != nil {
		return nil, err
	}
	return &proj, nil
}

func handleGetSchema(w http.ResponseWriter, r *http.Request) {
	proj, err := getProject()
	if err != nil {
		http.Error(w, fmt.Sprintf("读取 schema 文件失败: %v", err), http.StatusInternalServerError)
		return
	}
	// 将proj.Name设置为workspace的最后一级文件夹名字
	proj.Name = conf.Config.ProjectName

	out, err := proto.Marshal(proj)
	if err != nil {
		http.Error(w, fmt.Sprintf("序列化为 proto 失败: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

func handlePutSchema(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("读取请求体失败: %v", err), http.StatusBadRequest)
		return
	}
	var proj protopkg.Project
	if err := proto.Unmarshal(data, &proj); err != nil {
		http.Error(w, fmt.Sprintf("解析 proto 数据失败: %v", err), http.StatusBadRequest)
		return
	}

	jsonBytes, err := protojson.Marshal(&proj)
	if err != nil {
		http.Error(w, fmt.Sprintf("转换为 JSON 失败: %v", err), http.StatusInternalServerError)
		return
	}

	prettyJsonBytes, err := PrettyJSON(jsonBytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("JSON 格式化失败: %v", err), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(latestVersionFile, prettyJsonBytes, 0644); err != nil {
		http.Error(w, fmt.Sprintf("保存 schema 文件失败: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = crand.Read(b)
	return hex.EncodeToString(b)
}

func generatorExePath() string {
	return conf.Config.GeneratorPath
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	id := generateID()
	ch := make(chan string, 256)
	job := &generateJob{ch: ch}
	jobsMu.Lock()
	jobs[id] = job
	jobsMu.Unlock()

	go runGenerator(id, job)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func runGenerator(id string, job *generateJob) {
	defer func() {
		close(job.ch)
		jobsMu.Lock()
		delete(jobs, id)
		jobsMu.Unlock()
	}()

	exe := generatorExePath()
	cmd := exec.Command(exe, "-config", configPath)
	pr, pw, err := os.Pipe()
	if err != nil {
		job.ch <- fmt.Sprintf("error: %v\n", err)
		logger.Errorf("run generator: %v", err)
		return
	}
	cmd.Stdout = pw
	cmd.Stderr = pw
	if err := cmd.Start(); err != nil {
		pw.Close()
		job.ch <- fmt.Sprintf("error: %v\n", err)
		logger.Errorf("run generator: %v", err)
		return
	}
	go func() { cmd.Wait(); pw.Close() }()
	sc := bufio.NewScanner(pr)
	for sc.Scan() {
		job.ch <- sc.Text() + "\n"
	}
	pr.Close()
}

func handleGenerateStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	jobsMu.Lock()
	job, ok := jobs[id]
	if !ok {
		jobsMu.Unlock()
		http.Error(w, "job not found or already finished", http.StatusNotFound)
		return
	}
	delete(jobs, id)
	jobsMu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	for line := range job.ch {
		// SSE: 每行以 "data: " 前缀发送，多行则多组 data:
		const prefix = "data: "
		for len(line) > 0 {
			i := 0
			for i < len(line) && line[i] != '\n' {
				i++
			}
			fmt.Fprint(w, prefix, line[:i], "\n")
			if i < len(line) {
				line = line[i+1:]
			} else {
				line = ""
			}
		}
		fmt.Fprint(w, string("\n"))
		flusher.Flush()
	}
}

func PrettyJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "    "); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func handleSuggestAttrsAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST", http.StatusMethodNotAllowed)
		return
	}
	var req ai.SuggestAttrsAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("解析请求体失败: %v", err), http.StatusBadRequest)
		return
	}

	if conf.Config.LLMProvider == "" {
		http.Error(w, "未指定 LLM_PROVIDER,可选项：DEEPSEEK,DOUBAO", http.StatusBadRequest)
		return
	}

	project, err := getProject()
	if err != nil {
		http.Error(w, fmt.Sprintf("读取 schema 文件失败: %v", err), http.StatusInternalServerError)
		return
	}
	if !conf.Feature.LLM {
		http.Error(w, "未配置 LLM_API_KEY", http.StatusServiceUnavailable)
		return
	}
	llmAPIKey := conf.Config.LLMAPIKey
	switch conf.Config.LLMProvider {
	case "DEEPSEEK":
		data, err := ai.DeeoSeekTableAISuggestAttrs(conf.Config.LLMDeepseekChatCompletionsURL, conf.Config.LLMDeepseekModelID, llmAPIKey, project, &req)
		if err != nil {
			log.Printf("suggest_attrs_ai: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("suggest_attrs_ai encode: %v", err)
		}
	case "DOUBAO":
		data, err := ai.DouBaoTableAISuggestAttrs(conf.Config.LLMDoubaoChatCompletionsURL, conf.Config.LLMDoubaoModelID, llmAPIKey, project, &req)
		if err != nil {
			log.Printf("suggest_attrs_ai: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("suggest_attrs_ai encode: %v", err)
		}
		return
	case "QWEN":
		data, err := ai.QwenTableAISuggestAttrs(conf.Config.LLMQwenChatCompletionsURL, conf.Config.LLMQwenModelID, llmAPIKey, project, &req)
		if err != nil {
			log.Printf("suggest_attrs_ai: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("suggest_attrs_ai encode: %v", err)
		}
		return
	case "GLM":
		data, err := ai.GlmTableAISuggestAttrs(conf.Config.LLMGLMChatCompletionsURL, conf.Config.LLMGLMModelID, llmAPIKey, project, &req)
		if err != nil {
			log.Printf("suggest_attrs_ai: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("suggest_attrs_ai encode: %v", err)
		}
		return
	case "OPENAI":
		data, err := ai.OpenAITableAISuggestAttrs(conf.Config.LLMOpenaiChatCompletionsURL, conf.Config.LLMOpenaiModelID, llmAPIKey, project, &req)
		if err != nil {
			log.Printf("suggest_attrs_ai: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("suggest_attrs_ai encode: %v", err)
		}
		return
	case "CLAUDE":
		data, err := ai.ClaudeTableAISuggestAttrs(conf.Config.LLMClaudeChatCompletionsURL, conf.Config.LLMClaudeModelID, llmAPIKey, conf.Config.LLMClaudeVersion, conf.Config.LLMClaudeMaxTokens, project, &req)
		if err != nil {
			log.Printf("suggest_attrs_ai: %v", err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("suggest_attrs_ai encode: %v", err)
		}
		return
	default:
		http.Error(w, fmt.Sprintf("不支持的 LLM_PROVIDER: %s", conf.Config.LLMProvider), http.StatusBadRequest)
	}
}

func handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "仅支持 GET", http.StatusMethodNotAllowed)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "缺少查询参数 q 中文词", http.StatusBadRequest)
		return
	}

	if conf.Config.TransProvider == "" {
		http.Error(w, "未配置 TRANS_PROVIDER,可选项：BAIDU,ALIYUN,TENCENT", http.StatusServiceUnavailable)
		return
	}
	if !conf.Feature.Trans {
		http.Error(w, "未配置 TRANS_API_KEY 或 TRANS_API_SECRET", http.StatusServiceUnavailable)
		return
	}
	transAPIKey := conf.Config.TransAPIKey
	transAPISecret := conf.Config.TransAPISecret
	var err error
	var en string
	switch conf.Config.TransProvider {
	case "BAIDU":
		en, err = trans.BaiduTranslate(q, transAPIKey, transAPISecret)
	case "ALIYUN":
		en, err = trans.AliyunTranslate(q, transAPIKey, transAPISecret, conf.Config.TransAliyunAPIURL)
	case "TENCENT":
		en, err = trans.TencentTranslate(q, transAPIKey, transAPISecret, conf.Config.TransTencentAPIHost, conf.Config.TransTencentAPIVersion, conf.Config.TransTencentAPIAction, conf.Config.TransTencentAPIRegion)
	default:
		http.Error(w, fmt.Sprintf("不支持的 TRANS_PROVIDER: %s", conf.Config.TransProvider), http.StatusServiceUnavailable)
		return
	}

	if err != nil {
		log.Printf("translate: %v", err)
		http.Error(w, fmt.Sprintf("请求失败: %v", err), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"en": en})
}
