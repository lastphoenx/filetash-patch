package plg_backend_pcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/mickael-kerjean/filestash/server/common"
)

// version: 1.0.2
func init() {
	Backend.Register("pcloud", PCloud{})
}

type PCloud struct {
	ClientId       string
	ClientSecret   string
	Hostname       string
	Bearer         string
	ApiHost        string
	RootFolderid   string
	RootPath       string
}

func (p PCloud) getClientId() string {
	if v := os.Getenv("PCLOUD_CLIENT_ID"); v != "" {
		return v
	}
	return Config.Get("auth.pcloud.client_id").Default("").String()
}

func (p PCloud) getClientSecret() string {
	if v := os.Getenv("PCLOUD_CLIENT_SECRET"); v != "" {
		return v
	}
	return Config.Get("auth.pcloud.client_secret").Default("").String()
}

func (p PCloud) getHostname() string {
	if v := os.Getenv("APPLICATION_URL"); v != "" {
		v = strings.TrimPrefix(v, "https://")
		v = strings.TrimPrefix(v, "http://")
		return v
	}
	h := Config.Get("general.host").String()
	h = strings.TrimPrefix(h, "https://")
	h = strings.TrimPrefix(h, "http://")
	return h
}

func (p PCloud) getRedirectURI() string {
	return "https://" + p.getHostname() + "/login"
}

func (p PCloud) Init(params map[string]string, app *App) (IBackend, error) {
	backend := &PCloud{}
	backend.ClientId = p.getClientId()
	backend.ClientSecret = p.getClientSecret()
	backend.Hostname = p.getHostname()
	backend.Bearer = params["token"]
	backend.ApiHost = params["api_host"]
	if backend.ApiHost == "" {
		backend.ApiHost = "api.pcloud.com"
	}
	backend.RootFolderid = params["root_folderid"]
	backend.RootPath = params["root_path"]
	Log.Warning("pcloud Init: clientId_len=%d secret_len=%d hostname=%s apiHost=%s bearer_len=%d",
		len(backend.ClientId), len(backend.ClientSecret), backend.Hostname, backend.ApiHost, len(backend.Bearer))
	if backend.ClientId == "" {
		return backend, NewError("Missing ClientID: Contact your admin", 502)
	}
	if backend.ClientSecret == "" {
		return backend, NewError("Missing ClientSecret: Contact your admin", 502)
	}
	if backend.Hostname == "" {
		return backend, NewError("Missing Hostname: Contact your admin", 502)
	}
	return backend, nil
}

func (p PCloud) LoginForm() Form {
	return Form{
		Elmnts: []FormElement{
			{
				Name:  "type",
				Type:  "hidden",
				Value: "pcloud",
			},
			{
				ReadOnly: true,
				Name:     "oauth2",
				Type:     "hidden",
				Value:    "/api/session/auth/pcloud",
			},
			{
				ReadOnly: true,
				Name:     "image",
				Type:     "image",
				Value:    "data:image/svg+xml;utf8;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCA1MCA0MCI+PHBhdGggZmlsbD0iIzIwOThkOSIgZD0iTTI1IDVMMiAyMGg4djE1aDMwVjIwaDhMMjUgNXoiLz48L3N2Zz4=",
			},
		},
	}
}

func (p PCloud) OAuthURL() string {
	clientId := p.getClientId()
	redirectURI := p.getRedirectURI()
	Log.Warning("pcloud OAuthURL: clientId_len=%d redirectURI=%s", len(clientId), redirectURI)
	u := "https://my.pcloud.com/oauth2/authorize?"
	u += "client_id=" + clientId
	u += "&redirect_uri=" + url.QueryEscape(redirectURI)
	u += "&response_type=code"
	return u
}

func (p PCloud) OAuthToken(ctx *map[string]interface{}) error {
	// Log everything in ctx so we can see what Filestash passes in
	keys := make([]string, 0)
	for k := range *ctx {
		keys = append(keys, k)
	}
	Log.Warning("pcloud OAuthToken called: ctx_keys=%v clientId_len=%d secret_len=%d",
		keys, len(p.getClientId()), len(p.getClientSecret()))

	code := ""
	if str, ok := (*ctx)["code"].(string); ok {
		code = str
	}
	Log.Warning("pcloud OAuthToken: code_len=%d code_prefix=%.10s", len(code), code)

	if code == "" {
		Log.Warning("pcloud OAuthToken: ABORT - code is empty")
		return NewError("missing OAuth code", 400)
	}

	type tokenResponse struct {
		Result      int    `json:"result"`
		AccessToken string `json:"access_token"`
		Hostname    string `json:"hostname"`
		LocationId  int    `json:"locationid"`
		Error       string `json:"error"`
	}

	tryHost := func(host string) (*tokenResponse, error) {
		Log.Warning("pcloud OAuthToken: trying host=%s", host)
		params := url.Values{}
		params.Set("client_id", p.getClientId())
		params.Set("client_secret", p.getClientSecret())
		params.Set("code", code)
		resp, err := http.PostForm("https://"+host+"/oauth2_token", params)
		if err != nil {
			Log.Warning("pcloud OAuthToken: PostForm error host=%s err=%v", host, err)
			return nil, err
		}
		defer resp.Body.Close()
		var r tokenResponse
		json.NewDecoder(resp.Body).Decode(&r)
		Log.Warning("pcloud oauth2_token host=%s result=%d error=%s token_len=%d hostname=%s locationid=%d",
			host, r.Result, r.Error, len(r.AccessToken), r.Hostname, r.LocationId)
		return &r, nil
	}

	r, err := tryHost("eapi.pcloud.com")
	if err != nil || r.Result != 0 {
		Log.Warning("pcloud OAuthToken: eapi failed (err=%v result=%d), trying api.pcloud.com", err, func() int {
			if r != nil { return r.Result }
			return -1
		}())
		r, err = tryHost("api.pcloud.com")
		if err != nil {
			return err
		}
	}
	if r.Result != 0 {
		return NewError(fmt.Sprintf("pCloud OAuth error %d: %s", r.Result, r.Error), 401)
	}

	(*ctx)["token"] = r.AccessToken
	if v := os.Getenv("PCLOUD_ROOT_FOLDERID"); v != "" {
		(*ctx)["root_folderid"] = v
	}
	if v := os.Getenv("PCLOUD_ROOT_PATH"); v != "" {
		(*ctx)["root_path"] = v
	}
	if r.Hostname != "" {
		(*ctx)["api_host"] = r.Hostname
	} else if r.LocationId == 2 {
		(*ctx)["api_host"] = "eapi.pcloud.com"
	} else {
		(*ctx)["api_host"] = "api.pcloud.com"
	}
	delete(*ctx, "code")
	Log.Warning("pcloud OAuthToken: SUCCESS token_len=%d api_host=%s", len(r.AccessToken), (*ctx)["api_host"])
	return nil
}

func (p PCloud) apiURL(method string) string {
	return "https://" + p.ApiHost + "/" + method
}

func (p PCloud) get(method string, params map[string]string) (*http.Response, error) {
	q := url.Values{}
	q.Set("access_token", p.Bearer)
	for k, v := range params {
		if k != "path" {
			q.Set(k, v)
		}
	}
	rawURL := p.apiURL(method) + "?" + q.Encode()
	if pathVal, ok := params["path"]; ok {
		rawURL += "&path=" + strings.ReplaceAll(url.QueryEscape(pathVal), "%2F", "/")
	}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	return HTTPClient().Do(req)
}

func (p PCloud) Ls(path string) ([]os.FileInfo, error) {
	Log.Warning("pcloud Ls: path=%s apiHost=%s bearer_len=%d", path, p.ApiHost, len(p.Bearer))
	if uiRes, uiErr := p.get("userinfo", map[string]string{}); uiErr == nil {
		uiBody, _ := io.ReadAll(uiRes.Body)
		uiRes.Body.Close()
		Log.Warning("pcloud userinfo: %s", strings.ReplaceAll(string(uiBody), "\n", ""))
	}
	files := make([]os.FileInfo, 0)
	lsParams := map[string]string{"path": p.pcloudPath(path)}
	if path == "/" {
		lsParams = map[string]string{"folderid": "0"}
	}
	res, err := p.get("listfolder", lsParams)
	if err != nil {
		Log.Warning("pcloud Ls: get error=%v", err)
		return nil, err
	}
	rawBody, _ := io.ReadAll(res.Body)
	res.Body.Close()
	Log.Warning("pcloud Ls raw: %.800s", strings.ReplaceAll(string(rawBody), "\n", ""))
	var r struct {
		Result   int    `json:"result"`
		Error    string `json:"error"`
		Metadata struct {
			Contents []struct {
				Name     string `json:"name"`
				IsFolder bool   `json:"isfolder"`
				Size     int64  `json:"size"`
				Modified string `json:"modified"`
			} `json:"contents"`
		} `json:"metadata"`
	}
	json.Unmarshal(rawBody, &r)
	Log.Warning("pcloud Ls: result=%d error=%s items=%d", r.Result, r.Error, len(r.Metadata.Contents))
	if r.Result != 0 {
		return nil, NewError(r.Error, 400)
	}
	for _, obj := range r.Metadata.Contents {
		t, _ := time.Parse("Mon, 02 Jan 2006 15:04:05 +0000", obj.Modified)
		files = append(files, File{
			FName: obj.Name,
			FType: func(isFolder bool) string {
				if isFolder {
					return "directory"
				}
				return "file"
			}(obj.IsFolder),
			FTime: t.UnixNano() / 1000,
			FSize: obj.Size,
		})
	}
	return files, nil
}

func (p PCloud) Stat(path string) (os.FileInfo, error) {
	return nil, ErrNotImplemented
}

func (p PCloud) Cat(path string) (io.ReadCloser, error) {
	res, err := p.get("getfilelink", map[string]string{"path": p.pcloudPath(path)})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var r struct {
		Result int      `json:"result"`
		Error  string   `json:"error"`
		Hosts  []string `json:"hosts"`
		Path   string   `json:"path"`
	}
	json.NewDecoder(res.Body).Decode(&r)
	if r.Result != 0 {
		return nil, NewError(r.Error, 400)
	}
	if len(r.Hosts) == 0 {
		return nil, NewError("no download host returned", 500)
	}
	dlRes, err := http.Get("https://" + r.Hosts[0] + r.Path)
	if err != nil {
		return nil, err
	}
	return dlRes.Body, nil
}

func (p PCloud) Mkdir(path string) error {
	res, err := p.get("createfolder", map[string]string{"path": p.pcloudPath(path)})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var r struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	json.NewDecoder(res.Body).Decode(&r)
	if r.Result != 0 {
		return NewError(r.Error, 400)
	}
	return nil
}

func (p PCloud) Rm(path string) error {
	res, err := p.get("deletefile", map[string]string{"path": p.pcloudPath(path)})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var r struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	json.NewDecoder(res.Body).Decode(&r)
	if r.Result == 0 {
		return nil
	}
	res2, err := p.get("deletefolderrecursive", map[string]string{"path": p.pcloudPath(path)})
	if err != nil {
		return err
	}
	defer res2.Body.Close()
	var r2 struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	json.NewDecoder(res2.Body).Decode(&r2)
	if r2.Result != 0 {
		return NewError(r2.Error, 400)
	}
	return nil
}

func (p PCloud) Mv(from string, to string) error {
	toDir := filepath.Dir(p.pcloudPath(to))
	toName := filepath.Base(to)
	res, err := p.get("renamefile", map[string]string{
		"path": p.pcloudPath(from), "topath": toDir, "toname": toName,
	})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var r struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	json.NewDecoder(res.Body).Decode(&r)
	if r.Result == 0 {
		return nil
	}
	res2, err := p.get("renamefolder", map[string]string{
		"path": p.pcloudPath(from), "topath": toDir, "toname": toName,
	})
	if err != nil {
		return err
	}
	defer res2.Body.Close()
	var r2 struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	json.NewDecoder(res2.Body).Decode(&r2)
	if r2.Result != 0 {
		return NewError(r2.Error, 400)
	}
	return nil
}

func (p PCloud) Touch(path string) error {
	return p.Save(path, strings.NewReader(""))
}

func (p PCloud) Save(path string, file io.Reader) error {
	q := url.Values{}
	q.Set("access_token", p.Bearer)
	q.Set("path", filepath.Dir(p.pcloudPath(path)))
	q.Set("filename", filepath.Base(path))
	req, err := http.NewRequest("POST", p.apiURL("uploadfile")+"?"+q.Encode(), file)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var r struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Result != 0 {
		return NewError(fmt.Sprintf("pCloud upload error %d: %s", r.Result, r.Error), 400)
	}
	return nil
}

func (p PCloud) pcloudPath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}
