package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
)

var resultsTemplate = `<html>
<head>
   <meta http-equiv="Content-Type" content="text/html; charset=utf-8">
   <title>e2e test results</title>
</head>
<body>
   {{ . }}
</body>
</html>`

func main() {
	// // Open our jsonFile
	// jsonFile, err := os.Open("results_detailed_skip_prefix.json")
	// // if we os.Open returns an error then handle it
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// fmt.Println("Successfully Opened json file")
	// // defer the closing of our jsonFile so that we can parse it later on
	// defer jsonFile.Close()

	// byteValue, _ := ioutil.ReadAll(jsonFile)

	// var results interface{}
	// err = json.Unmarshal([]byte(byteValue), &results)
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// fmt.Println(results)

	http.Handle("/", http.FileServer(http.Dir(".")))
	// http.HandleFunc("/", report_result)
	// http.Handle("/file/", http.StripPrefix("/file", http.FileServer(http.Dir("."))))

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
}

func report_result(w http.ResponseWriter, req *http.Request) {
	file, _ := ioutil.ReadFile("results_report.txt")

	tpl := template.Must(template.New("h").Parse(resultsTemplate))
	tpl.Execute(w, string(file))
}
