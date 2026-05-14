package ai

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/light2000/laravel-modeler-studio/logger"
	"github.com/light2000/laravel-modeler-studio/proto"
)

var (
	userAISuggestAttrsTemplate   *template.Template
	systemAISuggestAttrsTemplate *template.Template
)

func TplInit(promptTplBaseDir string) {
	userAISuggestAttrsTemplate = getTemplate(filepath.Join(promptTplBaseDir, "user_ai_suggest_attrs.tpl"))
	systemAISuggestAttrsTemplate = getTemplate(filepath.Join(promptTplBaseDir, "system_ai_suggest_attrs.tpl"))
}

func getTemplate(tplPath string) *template.Template {
	tpl, err := template.New(filepath.Base(tplPath)).ParseFiles(tplPath)
	if err != nil {
		panic(fmt.Errorf("parse template %q failed: %w", tplPath, err))
	}

	return tpl
}

func TableAISuggestAttrsPrompt(project *proto.Project, request *SuggestAttrsAIRequest) ([]Message, error) {
	var systemBuf bytes.Buffer
	var userBuf bytes.Buffer
	err := userAISuggestAttrsTemplate.Execute(&userBuf, map[string]interface{}{
		"Request": request,
		"Project": project,
	})
	if err != nil {
		logger.Errorf("get user ai suggest attrs prompt content failed: %v", err)
		return nil, err
	}

	err = systemAISuggestAttrsTemplate.Execute(&systemBuf, map[string]interface{}{
		"Request": request,
		"Project": project,
	})
	if err != nil {
		logger.Errorf("get system ai suggest attrs prompt content failed: %v", err)
		return nil, err
	}

	return []Message{
		{Role: "system", Content: systemBuf.String()},
		{Role: "user", Content: userBuf.String()},
	}, nil
}
