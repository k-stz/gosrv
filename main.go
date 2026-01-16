package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	// not needed with golang 1.25+
	// _ "go.uber.org/automaxprocs"
	//
)

type Comment struct {
	Author    string
	Content   template.HTML // to show stored XSS, else "string" is better
	CreatedAt time.Time
}

var (
	comments []Comment = []Comment{
		//{Author: "Bob", Content: "Nice website!", CreatedAt: time.Now()},
	}
)

func httpbin(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<h1>Go Server Processed you're request:</h1>"))
	w.Write([]byte("<br><b>Time now:</b> " + time.Now().String()))
	w.Write([]byte("<br><b>Method</b>: " + r.Method))
	// w.Header().Set("Content-Type", "text/html")
	// else browser will cache invokations to this handler!!!
	//w.Header().Set("Cache-Control", "no-store, must-revalidate")
	// Wow, this prints a pretty cool table!
	w.Write([]byte("<br><b>RequestURI</b>: " + r.RequestURI))
	w.Write([]byte("<br><b>Request Headers</b>:<br><table border='1'><tr><th>Header</th><th>Value</th></tr>"))
	// Collect and sort header names
	keys := make([]string, 0, len(r.Header))
	for name := range r.Header {
		keys = append(keys, name)
	}
	sort.Strings(keys)

	// Print headers in alphabetical order
	for _, name := range keys {
		values := r.Header[name]
		for _, value := range values {
			fmt.Fprintf(w,
				"<tr><td>%s</td><td>%s</td></tr>",
				name,
				value,
			)
		}
	}
	w.Write([]byte("</table>"))

	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Println("couldn't read body", string(body))
	}
	w.Write([]byte("<br><b>Body:</b><br>"))
	w.Write(body)
}

func foo(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<b>Foo invoked, it worked!</b>"))
	// 	w.Write([]byte("<details>
	//   <div>
	//     Contact: HTML Example
	//   </div>
	//   <div>
	//     <a href="mailto:html-example@example.com">Email</a>
	//   </div>
	// </details>"))

}

// HTMX refuses to make AJAX requests from `file://` and will throw the error "htmx:invalidPath"
// so we need to serve the inital file from this webserver

type BankAccount struct {
	Id      int
	Balance int
}

var (
	MyAccount BankAccount
)

// works
func accountTest(w http.ResponseWriter, r *http.Request) {
	str := fmt.Sprintf("<b>Account: %d Deposit: %d</b>", MyAccount.Id, MyAccount.Balance)
	w.Write([]byte(str))
}

func account(w http.ResponseWriter, r *http.Request) {
	// parse tmeplate
	tmplPath := filepath.Join("templates", "account.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}

	// here usually fetch accoutn account, by we're using the global one
	data := MyAccount

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "render error:"+err.Error(), http.StatusInternalServerError)
	}
}

// Because this Handler returns HTML with embeded hypermedia control
// this is a hypermedia API and the golang server a hypermedia server
// and this API truely RESTful!
func withdrawal(w http.ResponseWriter, r *http.Request) {
	MyAccount.Balance -= 5
	account(w, r)
}

func deposits(w http.ResponseWriter, r *http.Request) {
	MyAccount.Balance += 5
	account(w, r)
}

func testEditThing(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<form hx-put="/contact/1" hx-target="this" hx-swap="outerHTML">
  <div>
    <label>First Name</label>
    <input type="text" name="firstName" value="Joe">
  </div>
  <div class="form-group">
    <label>Last Name</label>
    <input type="text" name="lastName" value="Blow">
  </div>
  <div class="form-group">
    <label>Email Address</label>
    <input type="email" name="email" value="joe@blow.com">
  </div>
  <button class="btn" type="submit">Submit</button>
  <button class="btn" hx-get="/contact/1">Cancel</button>
</form>`))
}

func loggingDecorator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Request path:", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func proc(w http.ResponseWriter, r *http.Request) {
	gomaxprocs := strconv.Itoa(runtime.GOMAXPROCS(0))
	numCPU := strconv.Itoa(runtime.NumCPU())
	fmt.Fprintf(w, "<p>gomaxprocs=%s numCPU=%s</p>", gomaxprocs, numCPU)
}

func procLimit(w http.ResponseWriter, r *http.Request) {
	contents, err := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if err != nil {
		fmt.Fprintf(w, "<p>couldn't read file /sys/fs/cgroup/cpu.max: %s </p>", err)
	}
	fmt.Fprintf(w, "<p>cpu.max: %s</p>", contents)
}

var (
	loadMu  sync.Mutex
	workers []chan struct{} // each worker has its own stop channel
)

// This is great add logic like:
/*
- tracking total goroutines
- tracking full load cpus
- add the ability to increment and decrement the number of endlessly running goroutine!
  - this can be implemented using cancellation
- interactively snipped together a website tracking different aspect of this experiment
- like one showing /sys/fs/cgroup/cpu.stat. I think that should be passed into a container
*/

// burns one CPU core
func cpuBurner(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			// burn CPU
		}
	}
}

func loadIncrease(w http.ResponseWriter, r *http.Request) {
	loadMu.Lock()
	defer loadMu.Unlock()

	stopChan := make(chan struct{})
	workers = append(workers, stopChan)

	go cpuBurner(stopChan)

	w.Header().Set("HX-Trigger", "stateChanged")
	w.Write([]byte("Started 1 more CPU load goroutine\n"))
}

func loadDecrease(w http.ResponseWriter, r *http.Request) {
	loadMu.Lock()
	defer loadMu.Unlock()

	if len(workers) == 0 {
		w.Write([]byte("No load goroutines to stop\n"))
		return
	}

	// stop the last worker
	last := workers[len(workers)-1]
	close(last)
	workers = workers[:len(workers)-1]

	w.Header().Set("HX-Trigger", "stateChanged")
	w.Write([]byte("Stopped 1 CPU load goroutine\n"))
}

func startupMessages() {
	pid := os.Getpid()
	fmt.Printf("pid= %d\n", pid)
}

func loadStats(w http.ResponseWriter, r *http.Request) {
	loadMu.Lock()
	count := len(workers)
	loadMu.Unlock()

	w.Write([]byte(fmt.Sprintf("Active load goroutines: %d\n", count)))
}

func loadStatsView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	html := fmt.Sprintf(`
		<div>
			<strong>Active workers:</strong> %d
		</div>
	`, len(workers))

	w.Write([]byte(html))
}

func loadPage(w http.ResponseWriter, r *http.Request) {
	loadPageTmpl := filepath.Join("templates", "loadpage.html")
	tmpl, err := template.ParseFiles(loadPageTmpl)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, nil)
}

func threadsViewHandler(w http.ResponseWriter, r *http.Request) {
	// Metric name for active OS threads

	const metricName = "/sched/:goroutines"

	//  /sched/gomaxprocs:threads
	// /sched/gomaxprocs:threads /sched/goroutines:goroutines
	// /sched/goroutines:goroutines /sched/latencies:seconds
	// /sched/latencies:seconds /sched/pauses/stopping/gc:seconds
	// /sched/pauses/stopping/gc:seconds /sched/pauses/stopping/other:seconds
	// /sched/pauses/stopping/other:seconds /sched/pauses/total/gc:seconds
	// /sched/pauses/total/gc:seconds /sched/pauses/total/other:seconds
	// /sched/pauses/total/other:seconds /sync/mutex/wait/total:seconds
	// /sync/mutex/wait/total:seconds

	// Create a list of metrics to read
	samples := make([]metrics.Sample, 1)
	samples[0].Name = metricName

	// Read the samples once
	//metrics.Read(samples)
	allMetrics := metrics.All()

	for _, m := range allMetrics {
		io.WriteString(w, fmt.Sprintf("%s <br>", m.Name))

		fmt.Fprintf(w, "%s\n", m.Name)
	}

	// Extract the value and write to the response writer
	if samples[0].Value.Kind() == metrics.KindUint64 {
		threadCount := samples[0].Value.Uint64()
		// Format the output directly as a string to be injected into the DOM
		io.WriteString(w, fmt.Sprintf("%d", threadCount))
	} else {
		io.WriteString(w, fmt.Sprintf("Metric unavailable: %s", metricName))
	}
}

func threadsIncreaseHandler(w http.ResponseWriter, r *http.Request) {
	currentGomaxprocs := runtime.GOMAXPROCS(0)
	currentGomaxprocs++
	runtime.GOMAXPROCS(currentGomaxprocs)
	w.Header().Set("HX-Trigger", "stateChanged")
	io.WriteString(w, fmt.Sprintf("Started 1 more CPU load goroutine: %s", currentGomaxprocs))
}

func threadsDecreaseHandler(w http.ResponseWriter, r *http.Request) {
	currentGomaxprocs := runtime.GOMAXPROCS(0)
	currentGomaxprocs--
	runtime.GOMAXPROCS(currentGomaxprocs)
	w.Header().Set("HX-Trigger", "stateChanged")
	io.WriteString(w, fmt.Sprintf("Started 1 more CPU load goroutine: %s", currentGomaxprocs))
}

func xssExampleHandler(w http.ResponseWriter, r *http.Request) {
	xssPageTemplate := filepath.Join("templates", "xss-example.html")
	tmpl, err := template.ParseFiles(xssPageTemplate)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, nil)
}

func xssCommentHandler(w http.ResponseWriter, r *http.Request) {
	templatePath := filepath.Join("templates", "xss-comment.html")
	// html/template automatically does
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, comments)
}

func addCommentHandler(w http.ResponseWriter, r *http.Request) {
	// we use this htmx trigger to autoload the newly added comment!
	w.Header().Add("HX-Trigger", "commentsUpdate")
	// only accept POST method
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(r.FormValue("content"))
	if content == "" {
		http.Error(w, "comment cannot be empty", http.StatusBadRequest)
		return
	}

	comment := Comment{
		Author:    "Anonymous", // derive later from ID TOKEN!
		Content:   template.HTML(content),
		CreatedAt: time.Now(),
	}
	// actually racy, you should use a lock on the comments slice! Is this a perfect
	// example of this being a best fit for Mutex vs go channels?
	comments = append(comments, comment)

	// We're done here, we're not returning a body, all this endpoint does it mutate server
	// state. This post doesn't responsd with any HTML. For that the /comment endpoint is used!
	w.WriteHeader(http.StatusNoContent)
}

func popCommentHandler(w http.ResponseWriter, r *http.Request) {
	// we use this htmx trigger to autoload the newly added comment!
	w.Header().Add("HX-Trigger", "commentsUpdate")
	if !(len(comments) >= 1) {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	comments = comments[0 : len(comments)-1]
	w.WriteHeader(http.StatusNoContent)
}

// This handler is susceptible to XSS:
// example of XSS injecting an img:
// (when website is running on localhost:5000)
// http://localhost:5000/echo?q=%3Cimg%20src%3D%22https%3A%2F%2Fupload.wikimedia.org%2Fwikipedia%2Fcommons%2Fthumb%2F3%2F31%2FNetherlandwarf.jpg%2F960px-Netherlandwarf.jpg%22%20style%3D%22width%3A%20100px%22%3E
// basically just urlencode whatever script you want to use, and put it:
// http://localhost:5000/echo?q=<HERE>
func echoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	q := r.URL.Query().Get("q")

	// ⚠️ INTENTIONALLY VULNERABLE
	// Directly embedding unescaped user input into HTML
	fmt.Fprintf(w, `
		<div>
			<strong>You typed:</strong>
			%s
		</div>
	`, q)
}

// This function gathers all metrics that start with /sched/ and formats them into an HTML list.
func serveAllSchedMetrics(w http.ResponseWriter, r *http.Request) {
	// 1. Get all available metric descriptions
	descs := metrics.All()

	// 2. Filter for only the /sched/ metrics and prepare the sample slice
	var schedMetrics []string
	for _, desc := range descs {
		if strings.HasPrefix(desc.Name, "/sched/") {
			schedMetrics = append(schedMetrics, desc.Name)
		}
	}

	// Prepare a slice to hold the samples we want to read
	samples := make([]metrics.Sample, len(schedMetrics))
	for i, name := range schedMetrics {
		samples[i].Name = name
	}

	// 3. Read the values for all filtered metrics at once
	metrics.Read(samples)

	// 4. Format the results as an HTML Unordered List (UL) for the frontend
	fmt.Fprintf(w, "<ul>")
	for i, sample := range samples {
		// Safely extract the value based on its kind
		var valueStr string
		switch sample.Value.Kind() {
		case metrics.KindUint64:
			valueStr = fmt.Sprintf("%d", sample.Value.Uint64())
		case metrics.KindFloat64:
			valueStr = fmt.Sprintf("%.4f", sample.Value.Float64())
		case metrics.KindFloat64Histogram:
			valueStr = fmt.Sprintf("Histogram data (Go %s+)", "1.19") // Latencies are often histograms
		default:
			valueStr = "N/A or Unknown Kind"
		}

		// Print an HTML list item
		fmt.Fprintf(w, "<li><strong>%s</strong>: %s</li>\n", schedMetrics[i], valueStr)
	}
	fmt.Fprintf(w, "</ul>")
}

func ajaxExampleHandler(w http.ResponseWriter, r *http.Request) {
	templatePath := filepath.Join("templates", "ajax-example.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, nil)

}

func dateHandler(w http.ResponseWriter, r *http.Request) {
	date := time.Now()

	templatePath := filepath.Join("templates", "date.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, date)

}

func dateSiteHandler(w http.ResponseWriter, r *http.Request) {
	templatePath := filepath.Join("templates", "date-site.yaml")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	err = tmpl.Execute(w, nil)
}

func corsExampleHandler(w http.ResponseWriter, r *http.Request) {
	// disable CORS
	w.Header().Add("Access-Control-Allow-Origin", "*")

	templatePath := filepath.Join("templates", "cors-example.html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, nil)

}

// jsonHandler just returns some json for SOP tests
func jsonHandler(w http.ResponseWriter, r *http.Request) {
	// if r.Method != http.MethodGet {
	// 	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	// 	return
	// }
	// 1. Define the data
	data := struct {
		Age  int    `json:"age"`
		Name string `json:"name"`
	}{
		Age:  33,
		Name: "Jane Doe",
	}

	// 2. Set the Content-Type header BEFORE writing status or body
	w.Header().Set("Content-Type", "application/json")

	// 3. Set the status code
	w.WriteHeader(http.StatusOK)

	// 4. Encode directly to the response writer (Best Practice for efficiency)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
}

func main() {
	startupMessages()

	mux := http.NewServeMux()

	// serve statis files from ./static !
	fs := http.FileServer(http.Dir("./static/"))
	mux.Handle("/", fs)

	// testing mounting pvc there
	//nfs := http.FileServer(http.Dir("./nfs/"))
	mux.Handle("/nfs/", http.StripPrefix("/nfs/", http.FileServer(http.Dir("./nfs"))))
	fmt.Println("mount nfs at ./nfs, served at /nfs")

	mux.HandleFunc("/load", loadPage)
	mux.HandleFunc("/load/increase", loadIncrease)
	mux.HandleFunc("/load/decrease", loadDecrease)
	mux.HandleFunc("/load/stats-view", loadStatsView)

	mux.HandleFunc("/threads/view", threadsViewHandler)
	mux.HandleFunc("/threads/increase", threadsIncreaseHandler)
	mux.HandleFunc("/threads/decrease", threadsDecreaseHandler)

	mux.HandleFunc("/metrics/sched", serveAllSchedMetrics)

	mux.HandleFunc("/httpbin", httpbin)
	mux.HandleFunc("/foo", foo)

	MyAccount = BankAccount{
		Id:      12345,
		Balance: 100,
	}

	mux.HandleFunc("/accountTest", accountTest)

	mux.HandleFunc("/account", account)

	mux.HandleFunc("/account/12345/deposits", deposits)
	mux.HandleFunc("/account/12345/withdrawal", withdrawal)

	mux.HandleFunc("/contact/1/edit", testEditThing)

	mux.HandleFunc("/proc", proc)

	mux.HandleFunc("/proc/limit", procLimit)

	// Test XSS
	mux.HandleFunc("/xss", xssExampleHandler)
	mux.HandleFunc("/comments", xssCommentHandler)
	mux.HandleFunc("/comments/add", addCommentHandler)
	mux.HandleFunc("/comments/pop", popCommentHandler)

	mux.HandleFunc("/echo", echoHandler)

	mux.HandleFunc("/cors", corsExampleHandler)
	mux.HandleFunc("/ajax", ajaxExampleHandler)

	mux.HandleFunc("/date", dateHandler)
	mux.HandleFunc("/datesite", dateSiteHandler)

	mux.HandleFunc("/json", jsonHandler)

	loggingMux := loggingDecorator(mux)

	// oauth
	//SetupOauth(mux)

	// Listen port
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = ":5000"
	}
	fmt.Println("Listening on :", port)
	err := http.ListenAndServe(port, loggingMux)
	if err != nil {
		panic(err)
	}
	fmt.Println("Server shutdown.")
}
