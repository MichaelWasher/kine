package mustgather

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/k3s-io/kine/pkg/server"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

var ErrNotMultifile = errors.New("this file is not a valid multi object file")

type ResourceBinding struct {
	Endpoint     string `yaml:"endpoint"`
	YamlLocation string `yaml:"yaml_loc"`
	Kind         string `yaml:"kind"`
	// Compiled Values
	compiledRegex    *regexp.Regexp
	compiledTemplate *template.Template
}
type Resource struct {
	Name      string
	Namespace string
}

type ObjListFile struct {
	APIVersion string      `json:"apiVersion"`
	Items      []Item      `json:"items"`
	Kind       string      `json:"kind"`
	Metadata   interface{} `json:"metadata"`
}
type Item struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   map[string]interface{} `json:"metadata"`
	Spec       interface{}            `json:"spec,omitempty"`
	Status     interface{}            `json:"status"`
}

type MustGather struct {
	resourceBindings []ResourceBinding
}

func New() *MustGather {
	return &MustGather{}
}

func (mg *MustGather) Start(ctx context.Context) error {
	// TODO build an in-memory index of the Must-Gather for quicker lookups
	var err error
	gatherRoot, err = findMustGatherRoot(gatherRoot)
	if err != nil {
		return err
	}

	// Load up the index from resource-mapping.yaml
	// Ensure that there are checks for all
	yamlFile, err := ioutil.ReadFile(bindingFile)

	if err != nil {
		log.Printf("yamlFile.Get err   #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, &mg.resourceBindings)

	if err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	// Force-compileable Regex for later use
	for i := range mg.resourceBindings {
		mg.resourceBindings[i].compiledRegex = regexp.MustCompile(mg.resourceBindings[i].Endpoint)
		mg.resourceBindings[i].compiledTemplate = template.Must(template.New("filename").Parse(mg.resourceBindings[i].YamlLocation))
	}

	return nil
}

func (mg *MustGather) findRelevantResourceBinding(requestedEndpoint string) (resource *Resource, resourceBinding *ResourceBinding) {

	for bindIdx, _ := range mg.resourceBindings {
		res := mg.resourceBindings[bindIdx]

		log.Debugf("Checking request %q against %s", requestedEndpoint, res.Endpoint)

		if !res.compiledRegex.MatchString(requestedEndpoint) {
			log.Debugf("Request did not match")
			continue
		}

		log.Debugf("Request %q matches %q", requestedEndpoint, res.Endpoint)
		log.Debugf("%#v\n", res.compiledRegex.FindStringSubmatch(requestedEndpoint))

		// Request has been matched so values can be populated
		resource = &Resource{}
		resourceBinding = &res

		match := res.compiledRegex.FindStringSubmatch(requestedEndpoint)
		for i, regexKey := range res.compiledRegex.SubexpNames() {
			// NOTE: Skip the first group which is the whole string
			if i == 0 {
				continue
			}

			switch regexKey {
			case "namespace":
				resource.Namespace = match[i]
			case "name":
				resource.Name = match[i]
			default:
				log.Infof("unexpected value found with key %q and value %q.", regexKey, match[i])
			}
		}
		break

	}
	return
}

func (mg *MustGather) getResourceFilename(resource Resource, resourceBinding *ResourceBinding) (filename string) {
	var tpl bytes.Buffer

	if err := resourceBinding.compiledTemplate.Execute(&tpl, resource); err != nil {
		panic(err)
	}
	filename = filepath.Join(gatherRoot, tpl.String())

	return
}

func (mg *MustGather) Get(ctx context.Context, key string, revision int64) (revRet int64, kvRet *server.KeyValue, errRet error) {
	rev := int64(1)

	// Process Health Checks
	if key == healthEndpoint {
		kvRet = &server.KeyValue{
			Key:            key,
			CreateRevision: 1,
			ModRevision:    1,
			Value:          []byte(`{"health":"true"}`),
			Lease:          1,
		}
		return 1, kvRet, nil
	}

	log.Debugf("Get has been called on %s", key)

	// Process the Request Key and extract type, namespace, name
	resource, resBinding := mg.findRelevantResourceBinding(key)
	if resBinding == nil {
		//NOTE: Do not return errors for this otherwise KubeAPI will crash
		// TODO This should be info log when mroe supported list
		log.Debugf("Resource binding not found")
		return 2, nil, nil
	}
	if resource == nil {
		log.Debug("Resource not found")
		return 2, nil, nil
	}

	log.Debugf("Processing namespace %q", resource.Namespace)
	filename := mg.getResourceFilename(*resource, resBinding)

	// Filter the ObjListFile output
	items, err := processObjectFile(filename)
	if err != nil {
		log.Errorf("Error while collectiong Pod files. %v", err)
		return 1, nil, err
	}

	filteredObjects := []Item{}
	log.Infof("Items %+v", items)
	for _, item := range *items {

		itemName, ok := item.Metadata["name"]
		if !ok {
			log.Infof("item does not have a name value: %+v", item)
		}
		if ok && itemName == resource.Name {
			filteredObjects = append(filteredObjects, item)
		}
	}

	kvSlice := ConvertItemsToKeyValue(filteredObjects, key, 1)
	if len(kvSlice) == 0 {
		// TODO Confirm what ETCD does when no value found
		return rev, nil, fmt.Errorf("that value does not exist")
	}

	kvRet = kvSlice[0]

	return rev, kvRet, nil
}

func getAllNamespaces() (*[]string, error) {
	namespaceRoot := filepath.Join(gatherRoot, "namespaces/")

	fSplice, err := ioutil.ReadDir(namespaceRoot)
	if err != nil {
		log.Errorf("Error collecting namespaces. %v", err)
		return nil, err
	}

	namespaceList := []string{}
	for _, f := range fSplice {
		// Inspect the Pod Dir
		if f.IsDir() {
			namespaceList = append(namespaceList, f.Name())
		}
	}

	return &namespaceList, nil
}

// TODO replace the isDir
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}
func processObjectFolder(dirName string) (*[]Item, error) {
	// ProcessObjectFile for all files in the folder *.yaml

	fSplice, err := ioutil.ReadDir(dirName)

	if err != nil {
		log.Errorf("Error collecting subfiles. %v", err)
		return nil, err
	}

	filenames := []string{}
	for _, f := range fSplice {

		log.Infof("Checking file %q for directory", f.Name())
		// Inspect the Pod Dir
		if f.IsDir() {
			continue
		}
		log.Debugf("Adding filepath to processing list", filepath.Join(dirName, f.Name()))
		filenames = append(filenames, filepath.Join(dirName, f.Name()))
	}

	items := []Item{}
	for _, file := range filenames {
		log.Debugf("Processing objects in %q", file)
		tmpItems, err := processObjectFile(file)

		if err != nil {
			log.Infof("Unable to process object file %q, Error: %v", file, err)
			return nil, err
		}
		items = append(items, *tmpItems...)
	}
	log.Infof("Completed folder processing")
	return &items, nil
}

func processObjectFile(objFilename string) (*[]Item, error) {

	log.Debugf("Attempting to open %s", objFilename)

	if _, err := os.Stat(objFilename); errors.Is(err, os.ErrNotExist) {
		log.Debugf("File is not present. %s", objFilename)
		return nil, err
	}

	f, err := os.Open(objFilename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	b := new(strings.Builder)
	io.Copy(b, f)
	var mf ObjListFile

	// Attempt to unmarshall a multi-file
	err = yaml.Unmarshal([]byte(b.String()), &mf)
	if err != nil {
		log.Debugf("Unable to unmarshall multiobject file")
		return nil, err
	}
	if strings.Contains(mf.Kind, "List") {
		return &mf.Items, nil
	}

	// Not a Multifile; Unmarshal a single file
	var item Item
	err = yaml.Unmarshal([]byte(b.String()), &item)
	if err != nil {
		log.Debugf("Unable to unmarshall singleobject file")
		return nil, err
	}

	items := []Item{item}
	return &items, nil
}

func ConvertItemsToKeyValue(items []Item, key string, revision int64) []*server.KeyValue {
	var kvRet []*server.KeyValue

	for _, item := range items {
		marshalledString, err := json.Marshal(item)
		if err != nil {
			log.Errorf("Unable to re-marshal json. %v", item)
			continue
		}
		kvRet = append(kvRet, &server.KeyValue{
			Key:            key,
			CreateRevision: revision,
			ModRevision:    revision,
			Value:          marshalledString,
			Lease:          10,
		})
	}
	return kvRet
}

func (mg *MustGather) List(ctx context.Context, prefix, startKey string, limit, revision int64) (revRet int64, kvRet []*server.KeyValue, errRet error) {
	log.Debugf("List has been called with prefix %s", prefix)

	// Extract values from request
	resource, resBinding := mg.findRelevantResourceBinding(startKey)
	if resBinding == nil {
		//NOTE: Do not return errors for this otherwise KubeAPI will crash
		log.Info("Resource binding not found")
		return 2, nil, nil
	}
	if resource == nil {
		log.Info("Resource not found")
		return 2, nil, nil
	}

	// Populate Namespaces to search
	namespaces := []string{}
	// TODO This is not a good way to check if CR is Namespaced ... But works
	if resource.Namespace == "" && strings.Contains(resBinding.YamlLocation, "Namespace") {
		namespaceTmp, err := getAllNamespaces()
		if err != nil {
			log.Infof("Unable to collect list of namespaces. %v", err)
			return 1, nil, err
		}
		namespaces = *namespaceTmp
	} else {
		namespaces = append(namespaces, resource.Namespace)
	}

	// Iterate over namespaces and collect objects
	log.Debugf("Namespaces list: %v", namespaces)
	for _, namespace := range namespaces {

		// TODO this should only work for namespaces values
		log.Debugf("Processing namespace %q", namespace)
		filename := mg.getResourceFilename(Resource{Namespace: namespace}, resBinding)
		// Check for folder presence like with CRDs in MG
		var items *[]Item
		var err error

		isDir, err := isDirectory(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Debugf("File is missing for %s. Skipping", namespace)
				continue
			}
			return 1, nil, err
		}

		if isDir {
			items, err = processObjectFolder(filename)
		} else {
			items, err = processObjectFile(filename)
		}

		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Debugf("File is missing for %s. Skipping", namespace)
				continue
			}
			log.Errorf("Error while processing multi-file %v", err)
			return 1, nil, err
		}
		key := prefix + "/" + namespace
		kvRet = append(kvRet, ConvertItemsToKeyValue(*items, key, 1)...)
	}

	return 1, kvRet, nil
}

func findMustGatherRoot(providedGatherRoot string) (string, error) {

	fSplice, err := ioutil.ReadDir(providedGatherRoot)
	if err != nil {
		log.Errorf("Error finding must-gather in %q: %v", providedGatherRoot, err)
		return "", err
	}

	for _, f := range fSplice {
		// Inspect the Pod Dir
		if f.IsDir() {
			if strings.Contains(f.Name(), "quay-io-openshift-release-dev-ocp-v4-0-art-dev-sha256-") {
				return filepath.Join(providedGatherRoot, f.Name()), nil
			}
			if f.Name() == "namespaces" {
				return providedGatherRoot, nil
			}
		}
	}

	log.Errorf("Unable to locate must-gather in %q", providedGatherRoot)
	return "", fmt.Errorf("unable to locate must-gather in %q", providedGatherRoot)
}
