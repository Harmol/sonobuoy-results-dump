package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v2"
)

const (
	// StatusFailed is the key we base junit pass/failure off of and save into
	// our canonical results format.
	StatusFailed = "failed"

	// StatusPassed is the key we base junit pass/failure off of and save into
	// our canonical results format.
	StatusPassed = "passed"

	// StatusSkipped is the key we base junit pass/failure off of and save into
	// our canonical results format.
	StatusSkipped = "skipped"

	// StatusUnknown is the key we fallback to in our canonical results format
	// if another can not be determined.
	StatusUnknown = "unknown"

	// StatusTimeout is the key used when the plugin does not report results within the
	// timeout period. It will be treated as a failure (e.g. its parent will be marked
	// as a failure).
	StatusTimeout = "timeout"
)

type SonobuoyDumpResult struct {
	PluginResultSummarys   []PluginResultSummary
	ClusterSummary         ClusterSummary
	ClusterSummaryFilePath string
}

type PluginResultSummary struct {
	Plugin       Item
	Total        int
	StatusCounts map[string]int
	FailedList   []string
	FilePath     string
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
	{{ range .PluginResultSummarys }}
		<div class="plugin">
			<h4>Plugin: <a href="/file?file={{.FilePath}}">{{.Plugin.Name}}</a></h4>
			<h5>Status: {{.Plugin.Status}}</h5>
			<h5>Total: {{.Total}}</h5>
			<h5>Passed: {{ index .StatusCounts "passed" }}</h5>
			<h5>Failed: {{ index .StatusCounts "failed" }}</h5>
			<h5>Skipped: {{ index .StatusCounts "skipped" }}</h5>
			{{ if ne (len .FailedList) 0 }}
				<h5>Failed tests:</h5>
				{{ range .FailedList }}
					<h5>{{.}}</h5>
				{{ end }}
		    {{ else }}
			{{ end }}
		</div>
	{{ end }}

	<div class="healthSummary">
		<h4><a href="/file?file={{.ClusterSummaryFilePath}}">Run Details:</a></h4>
		<h5>API Server version: {{.ClusterSummary.APIVersion}}</h5>
		<h5>Node health: {{.ClusterSummary.NodeHealth.Healthy}}/{{.ClusterSummary.NodeHealth.Total}} ({{.ClusterSummary.NodeHealth|healthRate}})%</h5>
		{{ if lt .ClusterSummary.NodeHealth.Healthy .ClusterSummary.NodeHealth.Total }}
			<h5>Details for failed pods:</h5>
				<table border="1">
					<tbody align="left">
						<tr>
							<td><h5>Namespace</h5></td>
							<td><h5>Name</h5></td>
							<td><h5>Ready</h5></td>
							<td><h5>Reason</h5></td>
							<td><h5>Message</h5></td>
						</tr>
							{{ range .ClusterSummary.NodeHealth.Details }}
								{{ if eq .Healthy false }}
								<tr>
									<td>{{ .Namespace }}</td>
									<td>{{ .Name }}</td>
									<td>{{ .Ready }}</td>
									<td>{{ .Reason }}</td>
									<td>{{ .Message }}</td>
								</tr>
								{{ else }}
								{{ end }}
							{{ end }}
					</tbody>
				</table>
		{{ else }}
		{{ end }}
		<h5>Pods health: {{.ClusterSummary.PodHealth.Healthy}}/{{.ClusterSummary.PodHealth.Total}} ({{.ClusterSummary.PodHealth|healthRate}})%</h5>
		{{ if lt .ClusterSummary.PodHealth.Healthy .ClusterSummary.PodHealth.Total }}
			<h5>Details for failed pods:</h5>
				<table border="1">
					<tbody align="left">
						<tr>
							<td><h5>Namespace</h5></td>
							<td><h5>Name</h5></td>
							<td><h5>Ready</h5></td>
							<td><h5>Reason</h5></td>
							<td><h5>Message</h5></td>
						</tr>
							{{ range .ClusterSummary.PodHealth.Details }}
								{{ if eq .Healthy false }}
								<tr>
									<td>{{ .Namespace }}</td>
									<td>{{ .Name }}</td>
									<td>{{ .Ready }}</td>
									<td>{{ .Reason }}</td>
									<td>{{ .Message }}</td>
								</tr>
								{{ else }}
								{{ end }}
							{{ end }}
					</tbody>
				</table>
		{{ else }}
		{{ end }}
		<h4>Errors detected in files:</h4>
		<table border="0">
			{{ range $errorType, $logHitCount := .ClusterSummary.ErrorInfo }}
				<td colspan="2"><h4>{{$errorType}}:</h4></td>
				{{ range $log, $count := $logHitCount }}
					<tr>
					<td>{{$count}}</td>
					<td><a href="/file?file={{$log}}">{{$log}}</a></td>
					</tr>
				{{ end }}
			{{ end }}
		</table>
	</div>
</body>
</html>
`

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
			log.Println(err)
			return
		}
		fmt.Println(item.Name)

		statusCounts := map[string]int{}
		var failedList []string

		statusCounts, failedList = item.walkForSummary(statusCounts, failedList)

		total := 0
		for _, v := range statusCounts {
			total += v
		}

		pluginResultSummary := PluginResultSummary{
			Plugin:       item,
			Total:        total,
			StatusCounts: statusCounts,
			FailedList:   failedList,
			FilePath:     file,
		}

		result.PluginResultSummarys = append(result.PluginResultSummarys, pluginResultSummary)
	}

	// unmarshal cluster health summary (dump mode) result file.
	result.ClusterSummaryFilePath = sonobuoyFile
	byteValue, _ := ioutil.ReadFile(sonobuoyFile)
	err := yaml.Unmarshal([]byte(byteValue), &result.ClusterSummary)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("cluster health summary")

	// setup Handlers
	router := mux.NewRouter()
	router.HandleFunc("/", result.summaryHandler)
	router.HandleFunc("/tests/{plugin}", result.testsHandler)
	router.HandleFunc("/file", logHandler)

	err = http.ListenAndServe(":8080", router)
	if err != nil {
		log.Println(err)
		return
	}
}

// walk for summary of plugin status.
func (plugin *Item) walkForSummary(statusCounts map[string]int, failList []string) (map[string]int, []string) {
	if len(plugin.Items) > 0 {
		for _, item := range plugin.Items {
			statusCounts, failList = item.walkForSummary(statusCounts, failList)
		}
		return statusCounts, failList
	}

	statusCounts[plugin.Status]++

	if plugin.Status == StatusFailed || plugin.Status == StatusTimeout {
		failList = append(failList, plugin.Name)
	}

	return statusCounts, failList
}

// summaryHandler handles the summary view template.
func (result *SonobuoyDumpResult) summaryHandler(w http.ResponseWriter, r *http.Request) {
	tpl := template.Must(template.New("summary").Funcs(template.FuncMap{
		"healthRate": healthRate,
	}).Parse(summaryTemplate))
	tpl.Execute(w, result)
}

// testsHandler handles the template for every tests of a specific plugin.
func (result *SonobuoyDumpResult) testsHandler(w http.ResponseWriter, r *http.Request) {
	pathVariables := mux.Vars(r)
	queryPlugin := pathVariables["plugin"]
	log.Println(queryPlugin)

	var plugin Item
	plugin = result.PluginResultSummarys[0].Plugin
	for _, item := range result.PluginResultSummarys {
		if item.Plugin.Name == queryPlugin {
			plugin = item.Plugin
		}
	}

	tpl := template.Must(template.New("failed-tests").Parse(testTemplate))
	tpl.Execute(w, plugin)
}

// failedTestsHandler handles the template for every tests of a specific plugin.
// func (result *SonobuoyDumpResult) failedTestsHandler(w http.ResponseWriter, r *http.Request) {
// 	pathVariables := mux.Vars(r)
// 	log.Println(pathVariables)
// 	queryPlugin := pathVariables["plugin"]
// 	log.Println(queryPlugin)

// 	// var plugin Item
// 	// plugin = result.Items[0]
// 	// for _, item := range result.Items {
// 	// 	if item.Name == queryPlugin {
// 	// 		plugin = item
// 	// 	}
// 	// }

// 	tpl := template.Must(template.New("failed-tests").Parse(failedTestTemplate))
// 	tpl.Execute(w, plugin)
// }

// logHandler handles the health info log.
func logHandler(w http.ResponseWriter, r *http.Request) {
	param, ok := r.URL.Query()["file"]
	if !ok || len(param[0]) < 1 {
		log.Println("URL param 'file' is missing")
		return
	}

	filePath := param[0]
	log.Printf("Handle request with param \"file\"=%s\n", filePath)
	bytesBuf, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read file %s\n", filePath)
		return
	}

	w.Write(bytesBuf)
}

func healthRate(healthInfo HealthInfo) int {
	return 100 * healthInfo.Healthy / healthInfo.Total
}
