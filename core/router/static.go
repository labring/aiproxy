package router

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/labring/aiproxy/core/common/config"
	"github.com/labring/aiproxy/core/public"
	"github.com/sirupsen/logrus"
)

func SetStaticFileRouter(router *gin.Engine) {
	router.SetHTMLTemplate(
		template.Must(
			template.New("").Funcs(router.FuncMap).ParseFS(public.Templates, "templates/*"),
		),
	)

	if config.DisableWeb {
		router.GET("/", func(ctx *gin.Context) {
			ctx.HTML(http.StatusOK, "index.tmpl", gin.H{
				"URL":               "https://github.com/labring/aiproxy",
				"INITIAL_COUNTDOWN": 15,
			})
		})
		return
	}

	if config.WebPath == "" {
		routerFs, ok := public.Public.(fs.ReadDirFS)
		if !ok {
			panic(fmt.Sprintf("public fs type error: %T, %v", public.Public, public.Public))
		}
		err := initFSRouter(router, routerFs, ".")
		if err != nil {
			panic(err)
		}
		fs := http.FS(public.Public)
		router.NoRoute(newIndexNoRouteHandler(fs))
	} else {
		absPath, err := filepath.Abs(config.WebPath)
		if err != nil {
			panic(err)
		}
		logrus.Infof("frontend file path: %s", absPath)
		routerFs, ok := os.DirFS(absPath).(fs.ReadDirFS)
		if !ok {
			panic(fmt.Sprintf("public fs type error: %T, %v", public.Public, public.Public))
		}
		err = initFSRouter(router, routerFs, ".")
		if err != nil {
			panic(err)
		}
		router.NoRoute(newDynamicNoRouteHandler(http.Dir(absPath)))
	}
}

func checkNoRouteNotFound(path string) bool {
	if strings.HasPrefix(path, "/api") ||
		strings.HasPrefix(path, "/mcp") ||
		strings.HasPrefix(path, "/v1") {
		return true
	}
	return false
}

func newIndexNoRouteHandler(fs http.FileSystem) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		if checkNoRouteNotFound(ctx.Request.URL.Path) {
			http.NotFound(ctx.Writer, ctx.Request)
			return
		}
		ctx.FileFromFS("", fs)
	}
}

func newDynamicNoRouteHandler(fs http.FileSystem) func(ctx *gin.Context) {
	fileServer := http.StripPrefix("/", http.FileServer(fs))
	return func(c *gin.Context) {
		if checkNoRouteNotFound(c.Request.URL.Path) {
			http.NotFound(c.Writer, c.Request)
			return
		}

		f, err := fs.Open(c.Request.URL.Path)
		if err != nil {
			c.FileFromFS("", fs)
			return
		}
		f.Close()

		fileServer.ServeHTTP(c.Writer, c.Request)
	}
}

type staticFileFS interface {
	StaticFileFS(relativePath, filepath string, fs http.FileSystem) gin.IRoutes
}

func initFSRouter(e staticFileFS, f fs.ReadDirFS, path string) error {
	dirs, err := f.ReadDir(path)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		u, err := url.JoinPath(path, dir.Name())
		if err != nil {
			return err
		}
		if dir.IsDir() {
			err = initFSRouter(e, f, u)
			if err != nil {
				return err
			}
		} else {
			e.StaticFileFS(u, u, http.FS(f))
		}
	}
	return nil
}
