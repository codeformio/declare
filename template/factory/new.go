package factory

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/codeformio/declare/template"
	"github.com/codeformio/declare/template/javascript"
	"github.com/codeformio/declare/template/jsonnet"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Templater interface {
	Template(client.Reader, *template.Input) (*template.Output, error)
}

func New(src map[string]string) (Templater, error) {
	var lang string
	for filename, _ := range src {
		currentLang := language(filename)

		if lang == "" {
			lang = currentLang
			continue
		}

		if currentLang != lang {
			return nil, fmt.Errorf("found mixed languages, %v & %v, only one is supported at a time", currentLang, lang)
		}
	}

	switch lang {
	case langJSONNet:
		return &jsonnet.Templater{Files: src}, nil
	case langJavascript:
		return &javascript.Templater{Files: src}, nil
	default:
		return nil, errors.New("no supported languages found in source")
	}
}

const (
	langJavascript = "javascript"
	langJSONNet    = "jsonnet"
)

func language(filename string) string {
	return map[string]string{
		".js":        langJavascript,
		".jsonnet":   langJSONNet,
		".libsonnet": langJSONNet,
	}[filepath.Ext(filename)]
}
