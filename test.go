package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"
)

type SonobuoyDumpResult struct {
	Items          []Item
	ClusterSummary ClusterSummary
}

type Item struct {
	Name     string                 `json:"name" yaml:"name"`
	Status   string                 `json:"status" yaml:"status"`
	Metadata map[string]string      `json:"meta,omitempty" yaml:"meta,omitempty"`
	Details  map[string]interface{} `json:"details,omitempty" yaml:"details,omitempty"`
	Items    []Item                 `json:"items,omitempty" yaml:"items,omitempty"`
}

type ClusterSummary struct {
	NodeHealth HealthInfo `json:"node_health" yaml:"node_health"`
	PodHealth  HealthInfo `json:"pod_health" yaml:"pod_health"`
	APIVersion string     `json:"api_version" yaml:"api_version"`
	ErrorInfo  LogSummary `json:"error_summary" yaml:"error_summary"`
}

type HealthInfo struct {
	Total   int                 `json:"total_nodes" yaml:"total_nodes"`
	Healthy int                 `json:"healthy_nodes" yaml:"healthy_nodes"`
	Details []HealthInfoDetails `json:"details,omitempty" yaml:"details,omitempty"`
}

type HealthInfoDetails struct {
	Name      string `json:"name" yaml:"name"`
	Healthy   bool   `json:"healthy" yaml:"healthy"`
	Ready     string `json:"ready" yaml:"ready"`
	Reason    string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Message   string `json:"message,omitempty" yaml:"message,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
}

type LogSummary map[string]LogHitCounter

type LogHitCounter map[string]int

var testTemplate = `<html>
<head>
   <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
   <title>E2E Test</title>
</head>
<body>
	<table border="1">
		<caption>{{ .Name }}</caption>
		{{ range .Items }}
		{{ range .Items }}
		<tbody align="center">
			<tr>
				<td rowspan="2">Name</td>
				<td rowspan="2">Status</td>
				<td colspan="2">Details</td>
			</tr>
			<tr>
				<td>Failure</td>
				<td>System-out</td>
			</tr>
				{{ range .Items }}
					<tr>
						<td>{{ .Name }}</td>
						<td>{{ .Status }}</td>
						<td>{{ index .Details "failure" }}</td>
						<td>{{ index .Details "system-out" }}</td>
					</tr>
				{{ end }}
		</tbody>
		{{ end }}
		{{ end }}
	</table>
</body>
</html>`

var failedTestTemplate = `<html>
<head>
   <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
   <title>E2E Test</title>
</head>
<body>
	<table border="1">
		<caption>{{ .Name }}</caption>
		{{ range .Items }}
		{{ range .Items }}
		<tbody align="center">
			<tr>
				<td rowspan="2">Name</td>
				<td rowspan="2">Status</td>
				<td colspan="2">Details</td>
			</tr>
			<tr>
				<td>Failure</td>
				<td>System-out</td>
			</tr>
				{{ range .Items }}
					{{ if eq .Status "failed" }}
					<tr>
						<td>{{ .Name }}</td>
						<td>{{ .Status }}</td>
						<td>{{ index .Details "failure" }}</td>
						<td>{{ index .Details "system-out" }}</td>
					</tr>
					{{ else }}
					{{ end }}
				{{ end }}
		</tbody>
		{{ end }}
		{{ end }}
	</table>
</body>
</html>`

var summaryTemplate = `<html>
<head>
   <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
   <title>Sonobuoy Results Report</title>
</head>
<body>
	{{ range .Items }}
		<div class="plugin">
			<h5>Plugin: <a href="/tests/{{.Name}}">{{.Name}}</a></h5>
			<h5>Status: {{.Status}}</h5>
		<div>
	{{ end }}
</body>
</html>`

func main() {
	var result SonobuoyDumpResult
	pluginFiles := []string{"results_dump_e2e.yaml", "results_dump_systemd_logs.yaml"}
	sonobuoyFile := "results_dump_sonobuoy.yaml"

	// unmarshal plugin (dump mode) result files.
	for _, file := range pluginFiles {
		var item Item
		byteValue, _ := ioutil.ReadFile(file)
		err := yaml.Unmarshal([]byte(byteValue), &item)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println(item.Name)
		result.Items = append(result.Items, item)
	}

	// unmarshal cluster health summary (dump mode) result file.
	byteValue, _ := ioutil.ReadFile(sonobuoyFile)
	err := yaml.Unmarshal([]byte(byteValue), &result.ClusterSummary)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("cluster health summary")

	// setup Handlers
	router := mux.NewRouter()
	router.HandleFunc("/", result.summaryHandler)
	router.HandleFunc("/tests/{plugin}", result.testsHandler)

	err = http.ListenAndServe(":8000", router)
	if err != nil {
		fmt.Println(err)
		return
	}
}

// summaryHandler handles the summary view template.
func (result *SonobuoyDumpResult) summaryHandler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.New("summary").Parse(summaryTemplate))
	tpl.Execute(w, result)
}

// testsHandler handles the template for every tests of a specific plugin.
func (result *SonobuoyDumpResult) testsHandler(w http.ResponseWriter, r *http.Request) {
	pathVariables := mux.Vars(r)
	queryPlugin := pathVariables["plugin"]
	fmt.Println(queryPlugin)

	var plugin Item
	plugin = result.Items[0]
	for _, item := range result.Items {
		if item.Name == queryPlugin {
			plugin = item
		}
	}

	tpl := template.Must(template.New("failed-tests").Parse(testTemplate))
	tpl.Execute(w, plugin)
}

// failedTestsHandler handles the template for every tests of a specific plugin.
func (result *SonobuoyDumpResult) failedTestsHandler(w http.ResponseWriter, r *http.Request) {
	pathVariables := mux.Vars(r)
	queryPlugin := pathVariables["plugin"]
	fmt.Println(queryPlugin)

	var plugin Item
	plugin = result.Items[0]
	for _, item := range result.Items {
		if item.Name == queryPlugin {
			plugin = item
		}
	}

	tpl := template.Must(template.New("failed-tests").Parse(failedTestTemplate))
	tpl.Execute(w, plugin)
}
