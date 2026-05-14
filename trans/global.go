package trans

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
	HTTPClient = utils.NewHTTPClient(conf.Config.TransProxy, time.Second*10)
}
