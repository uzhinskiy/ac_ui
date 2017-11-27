package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"./conf"
)

var (
	configfile string
	HTTPAddr   string
	err        error
)

type CustItemJSON struct {
	Custname    string `json:"custname,omitempty"`
	Custemail   string `json:"custemail,omitempty"`
	Servicetype string `json:"servicetype,omitempty"`
}

type CustomersJSON map[string]CustItemJSON

func init() {

	flag.StringVar(&HTTPAddr, "http", "127.0.0.1:8080", "Address to listen for HTTP requests on")
	flag.StringVar(&configfile, "config", "main.cfg", "Read configuration from this file")
	flag.StringVar(&configfile, "c", "main.cfg", "Read configuration from this file (short)")
	flag.Parse()

	conf.Config = make(conf.ConfigType)
	err := conf.Config.Parse(configfile)

	checkError(err, 1)

	log.Println("Read from config ", len(conf.Config), " items")
}

func checkError(err error, fatal int) {
	if err != nil {
		if fatal == 1 {
			log.Fatal("Error: ", err)
		} else {
			log.Println("Error: ", err)
		}
	}
}

func getJson() (string, CustomersJSON) {
	var cj CustomersJSON

	indoc, err := exec.Command("/usr/bin/perl", conf.Config["get_json"]).Output()
	if err != nil {
		log.Println("ListenAndServe: error reading file: ", err)
	}

	err = json.Unmarshal(indoc, &cj)
	if err != nil {
		log.Println(err)
	}

	return string(indoc), cj
}

func generatePerl(cj CustomersJSON) error {
	var out string
	for k, v := range cj {
		out += "$customers->{\"" + k + "\"} = {servicetype=>q{" + v.Servicetype + "}, custname=>q{" + v.Custname + "},custemail=>q{" + v.Custemail + "}};\n"
	}
	custFile, err := os.OpenFile(conf.Config["customers"], os.O_WRONLY|os.O_CREATE, 0644)
	defer custFile.Close()
	custFile.Truncate(0)
	custFile.Seek(0, 0)
	if err != nil {
		log.Println(err)
		return err
	}

	_, err = custFile.WriteString(string(out))
	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func serviceRestart() error {
	out, err := exec.Command("systemctl", "restart", conf.Config["service"]).Output()
	log.Println("Service restarted with ", err, string(out))
	return err
}

func main() {
	logTo := os.Stderr

	if logTo, err = os.OpenFile(conf.Config["log_file"], os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600); err != nil {
		log.Fatal(err)
	}
	defer logTo.Close()
	log.SetOutput(logTo)

	http.HandleFunc("/", requestHandler)
	http.HandleFunc("/list", List)
	http.HandleFunc("/admin", Admin)
	http.HandleFunc("/login", Login)
	http.HandleFunc("/info", Info)
	http.HandleFunc("/update", Update)
	http.HandleFunc("/create", Create)
	http.HandleFunc("/dump/", Dump)
	log.Println("HTTP server listening on", HTTPAddr)

	err := http.ListenAndServe(HTTPAddr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

/* функция для обработки подключившихся клиентов */
func requestHandler(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path
	base := conf.Config["document_root"]
	/* если отсутствует запрос к конкретному файлу – показать индексный файл */
	if file == "/" {
		file = "/index.html"
	}
	code := http.StatusOK
	/* если не удалось загрузить нужный файл – показать сообщение о 404-ой ошибке */
	respFile, err := os.OpenFile(base+file, os.O_RDONLY, 0)
	if err != nil {
		log.Println(err)
		file = "/404.html"
		respFile, err = os.OpenFile(base+file, os.O_RDONLY, 0)
		code = http.StatusNotFound
	}
	/* считать содержимое файла */
	fi, err := respFile.Stat()
	contentType := mime.TypeByExtension(path.Ext(file))
	var bytes = make([]byte, fi.Size())
	respFile.Read(bytes)
	log.Println(r.Method, "\t", r.URL.Path, "\t", code, "\t", r.UserAgent())
	/* отправить его клиенту */
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Server", conf.Config["version"])
	w.Write(bytes)
}

func List(w http.ResponseWriter, r *http.Request) {
	custJSONs, _ := getJson()
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Server", conf.Config["version"])
	log.Println(r.Method, "\t", r.URL.Path, "\t", http.StatusOK, "\t", r.UserAgent())
	fmt.Fprint(w, fmt.Sprintf("%s", custJSONs))
}

func Info(w http.ResponseWriter, r *http.Request) {
	_, cj := getJson()
	queryValues := r.URL.Query()
	id := queryValues.Get("id")
	j, err := json.Marshal(cj[id])
	if err != nil {
		log.Println(err)
	}
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Server", conf.Config["version"])
	log.Println(r.Method, "\t", r.URL.RequestURI(), "\t", http.StatusOK, "\t", r.UserAgent())
	fmt.Fprint(w, fmt.Sprintf("%s", j))
}

func Update(w http.ResponseWriter, r *http.Request) {
	var new_cj CustItemJSON
	var err error

	_, cj := getJson()

	queryValues := r.PostFormValue
	id := queryValues("id")
	new_cj.Custname = queryValues("custname")
	new_cj.Custemail = queryValues("custemail")
	new_cj.Servicetype = queryValues("servicetype")

	if new_cj.Custname != "" && new_cj.Custemail != "" && new_cj.Servicetype != "-1" {
		cj[id] = new_cj
		err = generatePerl(cj)
		err = serviceRestart()
	} else {
		err = fmt.Errorf("error")
	}

	if err != nil {
		log.Println(err)
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Header().Set("Server", conf.Config["version"])
		log.Println(r.Method, "\t", r.URL.Path, "\t", http.StatusServiceUnavailable, "\t", r.UserAgent())
		fmt.Fprint(w, "<h1>Error while file saving</h1><a href='/'>back</a>")
	}
	log.Println(r.Method, "\t", r.URL.Path, "\t", http.StatusOK, "\t", r.UserAgent())
	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

func Create(w http.ResponseWriter, r *http.Request) {
	var new_cj CustItemJSON
	var err error

	_, cj := getJson()

	queryValues := r.PostFormValue
	id := queryValues("id")
	new_cj.Custname = queryValues("custname")
	new_cj.Custemail = queryValues("custemail")
	new_cj.Servicetype = queryValues("servicetype")

	if new_cj.Custname != "" && new_cj.Custemail != "" && new_cj.Servicetype != "-1" {
		cj[id] = new_cj
		err = generatePerl(cj)
		err = serviceRestart()
	} else {
		err = fmt.Errorf("error")
	}
	if err != nil {
		log.Println(err)
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.Header().Set("Server", conf.Config["version"])
		log.Println(r.Method, "\t", r.URL.Path, "\t", http.StatusServiceUnavailable, "\t", r.UserAgent())
		fmt.Fprint(w, "<h1>Error while file saving</h1><a href='/'>back</a>")
	}
	log.Println(r.Method, "\t", r.URL.Path, "\t", http.StatusOK, "\t", r.UserAgent())
	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

func Dump(w http.ResponseWriter, r *http.Request) {
	//queryValues := r.URL.Query()
	urlPart := strings.Split(r.URL.Path, "/")
	dumpWhat := urlPart[2]
	log.Println(r.Method, "\t", r.URL.Path, "\t", http.StatusOK, "\t", r.UserAgent())
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Server", conf.Config["version"])

	switch dumpWhat {
	case "config":
		{
			fmt.Fprint(w, "<pre><ul>")
			for k, cfg := range conf.Config {
				fmt.Fprintf(w, "<li>%s = %s</li>", k, cfg)
			}
			fmt.Fprint(w, "</ul></pre>")
		}
	case "log":
		{
			respFile, err := os.OpenFile(conf.Config["log_file"], os.O_RDONLY, 0)
			if err != nil {
				log.Println(err)
			}
			fi, err := respFile.Stat()
			if err != nil {
				log.Println(err)
			}
			var bytes = make([]byte, fi.Size())
			respFile.Read(bytes)

			fmt.Fprintf(w, "<pre>%s</pre>", bytes)
		}
	}
}

func Admin(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path
	base := conf.Config["document_root"]

	auth_cookie, _ := r.Cookie("host")
	fmt.Println(auth_cookie)

	/* если отсутствует запрос к конкретному файлу – показать индексный файл */
	if auth_cookie == nil {
		file = "/login.html"
	} else {
		file = "/admin.html"
	}
	code := http.StatusOK
	/* если не удалось загрузить нужный файл – показать сообщение о 404-ой ошибке */
	respFile, err := os.OpenFile(base+file, os.O_RDONLY, 0)
	if err != nil {
		log.Println(err)
		file = "/404.html"
		respFile, err = os.OpenFile(base+file, os.O_RDONLY, 0)
		code = http.StatusNotFound
	}
	/* считать содержимое файла */
	fi, err := respFile.Stat()
	contentType := mime.TypeByExtension(path.Ext(file))
	var bytes = make([]byte, fi.Size())
	respFile.Read(bytes)
	log.Println(r.Method, "\t", r.URL.Path, "\t", code, "\t", r.UserAgent())
	/* отправить его клиенту */
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Server", conf.Config["version"])
	w.Write(bytes)
}

func Login(w http.ResponseWriter, r *http.Request) {
	expiration := time.Now().Add(5 * time.Minute)
	cookie := http.Cookie{Name: "host", Value: r.RemoteAddr, Expires: expiration}
	http.SetCookie(w, &cookie)
}
