package keycloak

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"io/ioutil"
	"encoding/json"
	"k8s.io/client-go/rest"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1template"github.com/openshift/api/template/v1"
	//v1route "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"github.com/aerogear/keycloak-operator/pkg/util"
	//v1routeclient "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"

)

const (
	SSO_TEMPLATE_NAME = "sso72-x509-postgresql-persistent.json"
	SSO_TEMPLATE_PATH = "./deploy/template"
)


func install(h *Handler, namespace string, params map[string]string) error{
	template, err := getTemplateFromFile()
	if err != nil {
		return errors.Wrap(err, "Failed to get template")
	}
	template = fillInParameters(template, params)

	rawExtensions, err := postTemplate(h, template, namespace)

	objects := make([]runtime.Object, 0)

	for _, rawObj := range rawExtensions {
		res, err := util.LoadKubernetesResource(rawObj.Raw)
		if err != nil {
			return err
			//return nil, err
		}
		objects = append(objects, res)
	}

	return nil
}


func getTemplateFromFile()(*v1template.Template, error){
	templateFilePath, err := filepath.Abs(fmt.Sprintf("%v/%v", SSO_TEMPLATE_PATH, SSO_TEMPLATE_NAME))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get template path")
	}

	templateJson, _ := getDataFromFile(templateFilePath)
	var tem v1template.Template
	if err := json.Unmarshal(templateJson, &tem); err != nil{
		fmt.Println(err)
	}
	return &tem, nil

}

func fillInParameters(template *v1template.Template, parameters map[string]string) *v1template.Template{
	for i, param := range template.Parameters {
		if value, ok := parameters[param.Name]; ok {
			template.Parameters[i].Value = value
		}
	}
	return template
}

func getDataFromFile(path string) ([]byte, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func postTemplate(h *Handler, sourceTemplate *v1template.Template, namespace string) ([]runtime.RawExtension, error){
	resource, err := json.Marshal(sourceTemplate)
	if err != nil {
		return nil, err
	}

	config := rest.CopyConfig(h.kubeconfig)
	config.GroupVersion = &schema.GroupVersion{
		Group: "template.openshift.io",
		Version: "v1",
	}
	config.APIPath = "/apis"
	config.AcceptContentTypes = "application/json"
	config.ContentType = "application/json"
	config.NegotiatedSerializer = basicNegotiatedSerializer{}
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	restClient, err := rest.RESTClientFor(config)
	result := restClient.
		Post().
		Namespace(namespace).
		Body(resource).
		Resource("processedtemplates").
		Do()
	if result.Error() == nil {
		data, err := result.Raw()
		if err != nil {
			return nil, err
		}

		templ, err := util.LoadKubernetesResource(data)
		if err != nil {
			return nil, err
		}

		if v1Temp, ok := templ.(*v1template.Template); ok {
			return v1Temp.Objects, nil
		}
		logrus.Error("Wrong type returned by the server", templ)
		return nil, errors.New("wrong type returned by the server")
	}
	//if result.Error() == nil {
	//	data, err := result.Raw()
	//	if err != nil {
	//		return nil, err
	//	}
	//	var templ v1template.Template
	//	if err := json.Unmarshal(data, &templ); err != nil{
	//		return nil, errors.New("wrong type returned by the server")
	//	}
	//	return templ.Objects, nil
	//}
	return nil, errors.New("wrong type returned by the server")
}

func LoadKubernetesResource(jsonData []byte) error{
	u := unstructured.Unstructured{}
	err := u.UnmarshalJSON(jsonData)
	if err != nil {
		return  err
	}
	return nil
}

