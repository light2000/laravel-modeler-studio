package ai

import (
	"net/http"
	"time"

	"github.com/light2000/laravel-modeler-studio/conf"
	"github.com/light2000/laravel-modeler-studio/utils"
)

var (
	HTTPClient *http.Client
)

func Init() {
	HTTPClient = utils.NewHTTPClient(conf.Config.LLMProxy, time.Second*240)
}
