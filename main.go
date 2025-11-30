package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	// not needed with golang 1.25+
	// _ "go.uber.org/automaxprocs"
)

func httpbin(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<h1>Go Server Processed you're request:</h1><br>"))
	w.Write([]byte("<br><b>Method</b>:" + r.Method))
	w.Header().Set("Content-Type", "text/html")
	// Wow, this prints a pretty cool table!
	fmt.Fprintln(w, "Request Headers<br>:<table border='1'><tr><th>Header</th><th>Value</th></tr>")
	for name, values := range r.Header {
		for _, value := range values {
			fmt.Fprintf(w, "<tr><td>%s</td><td>%s</td></tr>", name, value)
		}
	}
	w.Write([]byte("<br><b>RequestURI</b>:" + r.RequestURI))

	w.Write([]byte("<br><b>Body:</b><br>"))
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Println("couldn't read body", string(body))
	}
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
	fmt.Fprintf(w, "<b>gomaxprocs = %s</b><hr><b>numCPU =  %s</b>", gomaxprocs, numCPU)
}

func startupMessages() {
	pid := os.Getpid()
	fmt.Printf("pid= %d\n", pid)
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

	loggingMux := loggingDecorator(mux)
	fmt.Println("Listening on :8080")
	err := http.ListenAndServe(":8080", loggingMux)
	if err != nil {
		panic(err)
	}
	fmt.Println("Server shutdown.")
}
