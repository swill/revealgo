package revealgo

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/jasonlvhit/gocron"
	"golang.org/x/oauth2/google"
	sheets "google.golang.org/api/sheets/v4"
)

var (
	sheet              *sheets.Service
	password_protected bool
	params             ServerParam
	server_hash        string
	passwords          []string
)

type Server struct {
	port int
}

type Path struct {
	File   string
	Prefix string
}

type ServerParam struct {
	Paths         []Path
	Theme         string
	OriginalTheme bool
	Transition    string
	CredsFile     string
	Spreadsheet   string
	Worksheet     string
	PassColumn    string
	ExpireColumn  string
}

func (server *Server) Serve(param ServerParam) {
	params = param // set the global params

	password_protected = false
	// only attempt to password protect if a 'Spreadsheet' is specified
	if params.Spreadsheet != "" && params.Worksheet != "" && params.CredsFile != "" {
		// set the required environment variable to authorize
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", params.CredsFile)

		// setup sheet(s)
		// details here: https://developers.google.com/identity/protocols/application-default-credentials
		client, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/spreadsheets")
		if err != nil {
			log.Fatalf("Unable to setup google client: %v", err)
		}

		sheet, err = sheets.New(client)
		if err != nil {
			log.Fatalf("Unable to retrieve Sheets Client: %v", err)
		}

		// setup the initial passwords variable
		err = updateCachedPasswords()
		if err != nil {
			log.Fatalf("Problem with the Google Sheets API: %s", err.Error())
		}

		// setup a background cron to update the locally cached passwords.
		gocron.Every(5).Minutes().Do(updateCachedPasswords)
		go gocron.Start()

		password_protected = true // logical control for password protection
	}

	// global server hash
	server_h := sha1.New()
	server_h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	server_hash = fmt.Sprintf("%x", server_h.Sum(nil))

	port := 3000
	if server.port > 0 {
		port = server.port
	}
	fmt.Printf("accepting connections at http://*:%d/\n", port)
	http.Handle("/", &rootHandler{})
	if password_protected {
		http.Handle("/login", &loginHandler{})
	}
	http.Handle("/revealjs/", &assetHandler{assetPath: "assets"})
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

type assetHandler struct {
	assetPath string
}

func (h *assetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filepath := h.assetPath + r.URL.Path
	data, err := Asset(filepath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w = setResponse(w, filepath, data)
}

type loginHandler struct{}

func (h *loginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet: // get the form in the browser
		data, err := Asset("assets/templates/login.html")
		if err != nil {
			log.Printf("error:%v", err)
			http.NotFound(w, r)
			return
		}
		tmpl := template.New("login_template")
		tmpl.Parse(string(data))
		if err != nil {
			log.Printf("error:%v", err)
			http.NotFound(w, r)
			return
		}
		err = tmpl.Execute(w, params)
		if err != nil {
			log.Fatalf("error:%v", err)
		}
	case http.MethodPost: // post the form to the server
		r.ParseForm() // always parse the form before trying to use the form data
		password := strings.TrimSpace(r.FormValue("password"))
		if validPassword(password) {
			// setup the session cookie...
			created := time.Now().UnixNano()
			created_c := http.Cookie{
				Name:     "created",
				Value:    fmt.Sprintf("%d", created),
				MaxAge:   60 * 60 * 24,
				Path:     "/",
				HttpOnly: true,
			}
			session_h := sha1.New()
			session_h.Write([]byte(fmt.Sprintf("%s:%s:%d", password, server_hash, created)))
			session_c := http.Cookie{
				Name:     "session",
				Value:    fmt.Sprintf("%x", session_h.Sum(nil)),
				MaxAge:   60 * 60 * 24,
				Path:     "/",
				HttpOnly: true,
			}
			http.SetCookie(w, &created_c)
			http.SetCookie(w, &session_c)
			http.Redirect(w, r, "/", http.StatusFound)
		} else {
			http.Redirect(w, r, "/login?error=invalid_pass", http.StatusFound)
		}
	default:
		http.Redirect(w, r, "/login?error=invalid_method", http.StatusFound)
	}
}

type rootHandler struct{}

func (h *rootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	path, err := filepath.Rel("./", "."+urlPath)
	if err != nil {
		log.Fatalf("error:%v", err)
	}
	_, err = os.Stat(path)
	if err == nil {
		data, err := ioutil.ReadFile(path)
		if err == nil {
			w = setResponse(w, path, data)
			return
		}
	}

	// validate cookie or redirect to login if password_protected
	// IMPORTANT: Must be done after asset loading to minimize google sheets load...
	if password_protected && !validCookie(r) {
		http.Redirect(w, r, "/login", http.StatusFound)
	}

	data, err := Asset("assets/templates/slide.html")
	if err != nil {
		log.Printf("error:%v", err)
		http.NotFound(w, r)
		return
	}
	tmpl := template.New("slide template")
	tmpl.Parse(string(data))
	if err != nil {
		log.Printf("error:%v", err)
		http.NotFound(w, r)
		return
	}
	err = tmpl.Execute(w, params)
	if err != nil {
		log.Fatalf("error:%v", err)
	}
}

func detectContentType(path string, data []byte) string {
	switch {
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	}
	return http.DetectContentType(data)
}

func setResponse(w http.ResponseWriter, path string, data []byte) http.ResponseWriter {
	mimeType := detectContentType(path, data)
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if _, err := w.Write(data); err != nil {
		log.Fatal("unable to write data.")
	}
	return w
}

// -------------------------------------
// Sheets and session management...
// -------------------------------------

func validPassword(p string) bool {
	for _, pass := range passwords {
		if pass == strings.TrimSpace(p) {
			return true
		}
	}
	return false
}

func validCookie(r *http.Request) bool {
	created, c_err := r.Cookie("created")
	session, s_err := r.Cookie("session")
	if c_err == nil && s_err == nil {
		for _, pass := range passwords {
			// hash the password to check if it matches the client hash
			session_h := sha1.New()
			session_h.Write([]byte(fmt.Sprintf("%s:%s:%s", pass, server_hash, created.Value)))
			if session.Value == fmt.Sprintf("%x", session_h.Sum(nil)) {
				return true
			}
		}
	}
	return false
}

func updateCachedPasswords() error {
	// use a tmp var so validation doesn't fail while this function is running...
	tmp_passwords := []string{}

	// get the passwords
	password_resp, err := sheet.Spreadsheets.Values.Get(
		params.Spreadsheet,
		fmt.Sprintf("%s!%s:%s", params.Worksheet, params.PassColumn, params.PassColumn)).Do()
	if err != nil {
		return err
	}

	// get the expires details
	expires_resp, err := sheet.Spreadsheets.Values.Get(
		params.Spreadsheet,
		fmt.Sprintf("%s!%s:%s", params.Worksheet, params.ExpireColumn, params.ExpireColumn)).Do()
	if err != nil {
		return err
	}

	// add the valid passwords to the tmp var
	if len(password_resp.Values) > 0 {
		for r, row := range password_resp.Values {
			if r > 1 { // don't use heading row(s)
				expired := false
				if len(expires_resp.Values) > r && len(expires_resp.Values[r]) > 0 { // zero based array index vs len
					expires, err := time.Parse("2006-01-02", expires_resp.Values[r][0].(string))
					if err == nil && expires.AddDate(0, 0, 1).Before(time.Now().Local()) {
						expired = true
					}
				}
				if !expired {
					tmp_passwords = append(tmp_passwords, strings.TrimSpace(row[0].(string)))
				}
			}
		}
	}
	passwords = tmp_passwords
	return nil
}
