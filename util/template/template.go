package template

import (
	"bytes"
	"text/template"
	"github.com/golang/glog"
	"github.com/huxos/kube2dyups/proxy"
	"path"
)

type TemplateData struct {
	RouteTable map[string]*proxy.ServiceUnit
}

// Returns string content of a rendered template
func RenderUpstreamsTemplate(templateName, templateDir string, data interface{}) ([]byte, error) {

	tpl := template.New(templateName)

	if _, err := tpl.ParseFiles(
		path.Join(templateDir, "upstreams.conf.tmpl"),
		path.Join(templateDir, "upstream.tmpl"),
	); err != nil {
		glog.V(4).Infof("parse template file err %v", err)
		return nil, err
	}

	strBuffer := new(bytes.Buffer)

	if err := tpl.ExecuteTemplate(strBuffer, "upstreams.conf.tmpl", data); err != nil {
		glog.V(4).Infof("render upstreams err %v", err)
		return nil, err
	}

	return strBuffer.Bytes(), nil
}

func RenderUpstreamTemplate(templateName, templateDir string, data interface{}) ([]byte, error) {

	tpl := template.New(templateName)

	if _, err := tpl.ParseFiles(
		path.Join(templateDir, "upstream.tmpl"),
	); err != nil {
		glog.V(4).Infof("parse template file err %v", err)
		return nil, err
	}

	strBuffer := new(bytes.Buffer)

	if err := tpl.ExecuteTemplate(strBuffer, "upstream.tmpl", data); err != nil {
		glog.V(4).Infof("render upstream file err %v with %v", err, data)
		return nil, err
	}

	return strBuffer.Bytes(), nil
}
