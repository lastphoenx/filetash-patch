package plg_backend_pcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/mickael-kerjean/filestash/server/common"
)

// version: 1.1.4
func init() {
	Backend.Register("pcloud", PCloud{})
}

type PCloud struct {
	ClientId     string
	ClientSecret string
	Hostname     string
	Bearer       string
	ApiHost      string
	RootFolderid string
	RootPath     string
}

type pcloudMeta struct {
	Name     string
	IsFolder bool
	Size     int64
	Modified string
	FileId   int64
	FolderId int64
	Path     string
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
	Log.Warning("pcloud Init: clientId_len=%d secret_len=%d hostname=%s apiHost=%s bearer_len=%d rootPath=%s rootFolderid=%s",
		len(backend.ClientId), len(backend.ClientSecret), backend.Hostname, backend.ApiHost, len(backend.Bearer),
		backend.RootPath, backend.RootFolderid)
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
			if r != nil {
				return r.Result
			}
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

func normRemotePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

func pathDir(p string) string {
	p = strings.TrimSuffix(normRemotePath(p), "/")
	if p == "" || p == "/" {
		return "/"
	}
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return "/"
	}
	return p[:i]
}

func pathBase(p string) string {
	p = strings.TrimSuffix(normRemotePath(p), "/")
	if p == "" || p == "/" {
		return ""
	}
	i := strings.LastIndex(p, "/")
	return p[i+1:]
}

func parseBoolish(v interface{}) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int64:
		return x != 0
	case string:
		return x == "true" || x == "1"
	}
	return false
}

func parseInt64ish(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}

func parseModifiedUnix(modified string) int64 {
	t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 +0000", modified)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func isFolderItem(item map[string]interface{}) bool {
	if parseBoolish(item["isfolder"]) {
		return true
	}
	icon, _ := item["icon"].(string)
	return icon == "folder"
}

func metaFromMap(item map[string]interface{}, parentPath string) *pcloudMeta {
	name, _ := item["name"].(string)
	isFolder := isFolderItem(item)
	modified, _ := item["modified"].(string)
	itemPath, _ := item["path"].(string)
	if itemPath == "" && name != "" {
		if parentPath == "/" {
			itemPath = "/" + name
		} else {
			itemPath = parentPath + "/" + name
		}
	}
	return &pcloudMeta{
		Name:     name,
		IsFolder: isFolder,
		Size:     parseInt64ish(item["size"]),
		Modified: modified,
		FileId:   parseInt64ish(item["fileid"]),
		FolderId: parseInt64ish(item["folderid"]),
		Path:     normRemotePath(itemPath),
	}
}

func fileFromMap(item map[string]interface{}, parentPath string) File {
	meta := metaFromMap(item, parentPath)
	fPath := meta.Path
	if meta.IsFolder && fPath != "" && !strings.HasSuffix(fPath, "/") {
		fPath += "/"
	}
	fType := "file"
	if meta.IsFolder {
		fType = "directory"
	}
	return File{
		FName: meta.Name,
		FType: fType,
		FTime: parseModifiedUnix(meta.Modified),
		FSize: meta.Size,
		FPath: fPath,
	}
}

func parseListfolderBody(rawBody []byte) ([]map[string]interface{}, int, string, error) {
	var r struct {
		Result   int                    `json:"result"`
		Error    string                 `json:"error"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(rawBody, &r); err != nil {
		return nil, 0, "", err
	}
	if r.Result != 0 {
		return nil, r.Result, r.Error, NewError(r.Error, 400)
	}
	contents := make([]map[string]interface{}, 0)
	rawContents, ok := r.Metadata["contents"].([]interface{})
	if !ok {
		return contents, 0, "", nil
	}
	for _, entry := range rawContents {
		if item, ok := entry.(map[string]interface{}); ok {
			contents = append(contents, item)
		}
	}
	return contents, 0, "", nil
}

func (p PCloud) pcloudPath(path string) string {
	return normRemotePath(path)
}

func (p PCloud) apiURL(method string) string {
	return "https://" + p.ApiHost + "/" + method
}

func encodePathParam(key, value string) string {
	return key + "=" + strings.ReplaceAll(url.QueryEscape(value), "%2F", "/")
}

func (p PCloud) get(method string, params map[string]string) (*http.Response, error) {
	q := url.Values{}
	q.Set("access_token", p.Bearer)
	pathParams := make(map[string]string)
	for k, v := range params {
		if k == "path" || k == "topath" {
			pathParams[k] = v
		} else {
			q.Set(k, v)
		}
	}
	rawURL := p.apiURL(method) + "?" + q.Encode()
	for k, v := range pathParams {
		rawURL += "&" + encodePathParam(k, v)
	}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	return HTTPClient().Do(req)
}

func (p PCloud) lsParams(path string) map[string]string {
	if path == "/" || path == "" {
		if p.RootFolderid != "" {
			return map[string]string{"folderid": p.RootFolderid}
		}
		return map[string]string{"folderid": "0"}
	}
	remote := normRemotePath(strings.TrimSuffix(p.pcloudPath(path), "/"))
	if strings.HasSuffix(strings.TrimSpace(path), "/") {
		if meta, err := p.statRemote(remote); err == nil && meta.IsFolder && meta.FolderId > 0 {
			return map[string]string{"folderid": strconv.FormatInt(meta.FolderId, 10)}
		}
	}
	return map[string]string{"path": remote}
}

func (p PCloud) statRemote(remotePath string) (*pcloudMeta, error) {
	res, err := p.get("stat", map[string]string{"path": remotePath})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	rawBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var r struct {
		Result   int                    `json:"result"`
		Error    string                 `json:"error"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(rawBody, &r); err != nil {
		return nil, err
	}
	if r.Result != 0 {
		return nil, NewError(r.Error, 400)
	}
	meta := metaFromMap(r.Metadata, pathDir(remotePath))
	if meta.Path == "" {
		meta.Path = remotePath
	}
	return meta, nil
}

func (p PCloud) resolveEntry(path string) (*pcloudMeta, error) {
	wantFolder := strings.HasSuffix(strings.TrimSpace(path), "/")
	remotePath := normRemotePath(strings.TrimSuffix(p.pcloudPath(path), "/"))
	parent := pathDir(remotePath)
	name := pathBase(remotePath)
	if name == "" {
		return nil, NewError("invalid path", 400)
	}

	res, err := p.get("listfolder", map[string]string{"path": parent})
	if err != nil {
		return nil, err
	}
	rawBody, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	contents, result, errMsg, err := parseListfolderBody(rawBody)
	if err != nil {
		return nil, err
	}
	if result != 0 {
		return nil, NewError(errMsg, 400)
	}
	for _, item := range contents {
		meta := metaFromMap(item, parent)
		if meta.Name != name || meta.IsFolder != wantFolder {
			continue
		}
		if meta.Path == "" {
			meta.Path = remotePath
		}
		return meta, nil
	}
	return nil, NewError("file not found", 404)
}

func metaToFileInfo(meta *pcloudMeta) File {
	fType := "file"
	if meta.IsFolder {
		fType = "directory"
	}
	return File{
		FName: meta.Name,
		FType: fType,
		FTime: parseModifiedUnix(meta.Modified),
		FSize: meta.Size,
	}
}

func (p PCloud) ensurePath(remotePath string) error {
	remotePath = normRemotePath(remotePath)
	if remotePath == "/" {
		return nil
	}
	parts := strings.Split(strings.Trim(remotePath, "/"), "/")
	cur := ""
	for _, part := range parts {
		cur += "/" + part
		res, err := p.get("listfolder", map[string]string{"path": cur})
		if err != nil {
			return err
		}
		var r struct {
			Result int    `json:"result"`
			Error  string `json:"error"`
		}
		json.NewDecoder(res.Body).Decode(&r)
		res.Body.Close()
		if r.Result == 0 {
			continue
		}
		res2, err := p.get("createfolder", map[string]string{"path": cur})
		if err != nil {
			return err
		}
		var r2 struct {
			Result int    `json:"result"`
			Error  string `json:"error"`
		}
		json.NewDecoder(res2.Body).Decode(&r2)
		res2.Body.Close()
		if r2.Result == 0 || r2.Result == 2004 {
			continue
		}
		return NewError(r2.Error, 400)
	}
	return nil
}

func (p PCloud) Ls(path string) ([]os.FileInfo, error) {
	parentPath := normRemotePath(strings.TrimSuffix(p.pcloudPath(path), "/"))
	res, err := p.get("listfolder", p.lsParams(path))
	if err != nil {
		return nil, err
	}
	rawBody, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	contents, result, errMsg, err := parseListfolderBody(rawBody)
	if err != nil {
		return nil, err
	}
	if result != 0 {
		return nil, NewError(errMsg, 400)
	}
	files := make([]os.FileInfo, 0, len(contents))
	for _, item := range contents {
		files = append(files, fileFromMap(item, parentPath))
	}
	return files, nil
}

func (p PCloud) Stat(path string) (os.FileInfo, error) {
	meta, err := p.statRemote(p.pcloudPath(path))
	if err != nil {
		return nil, err
	}
	f := metaToFileInfo(meta)
	return f, nil
}

func (p PCloud) Cat(path string) (io.ReadCloser, error) {
	if strings.HasSuffix(strings.TrimSpace(path), "/") {
		return nil, NewError("is a directory", 400)
	}
	meta, err := p.resolveEntry(path)
	if err != nil {
		return nil, err
	}
	if meta.IsFolder {
		return nil, NewError("is a directory", 400)
	}
	params := map[string]string{}
	if meta.FileId > 0 {
		params["fileid"] = strconv.FormatInt(meta.FileId, 10)
	} else {
		params["path"] = meta.Path
	}
	res, err := p.get("getfilelink", params)
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
	dlURL := "https://" + r.Hosts[0] + r.Path
	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		return nil, err
	}
	dlRes, err := HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	return dlRes.Body, nil
}

func (p PCloud) apiResult(res *http.Response) (int, string, error) {
	defer res.Body.Close()
	var r struct {
		Result int    `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return 0, "", err
	}
	if r.Result != 0 {
		return r.Result, r.Error, NewError(r.Error, 400)
	}
	return 0, "", nil
}

func (p PCloud) deleteFileEntry(meta *pcloudMeta) error {
	params := map[string]string{}
	if meta.FileId > 0 {
		params["fileid"] = strconv.FormatInt(meta.FileId, 10)
	} else if meta.Path != "" {
		params["path"] = meta.Path
	} else {
		return NewError("missing file id", 400)
	}
	res, err := p.get("deletefile", params)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var r struct {
		Result   int    `json:"result"`
		Error    string `json:"error"`
		Metadata struct {
			IsDeleted bool `json:"isdeleted"`
		} `json:"metadata"`
	}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return err
	}
	if r.Result != 0 {
		return NewError(r.Error, 400)
	}
	return nil
}

func (p PCloud) deleteFolderEntry(meta *pcloudMeta) error {
	params := map[string]string{}
	if meta.FolderId > 0 {
		params["folderid"] = strconv.FormatInt(meta.FolderId, 10)
	} else if meta.Path != "" {
		params["path"] = meta.Path
	} else {
		return NewError("missing folder id", 400)
	}
	res, err := p.get("deletefolder", params)
	if err != nil {
		return err
	}
	_, _, err = p.apiResult(res)
	return err
}

// trashFolder moves a folder and all contents to the pCloud Trash (deletefile/deletefolder),
// never deletefolderrecursive which permanently removes data.
func (p PCloud) trashFolder(remotePath string) error {
	res, err := p.get("listfolder", map[string]string{"path": remotePath})
	if err != nil {
		return err
	}
	rawBody, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return err
	}
	contents, result, errMsg, err := parseListfolderBody(rawBody)
	if err != nil {
		return err
	}
	if result != 0 {
		return NewError(errMsg, 400)
	}
	for _, item := range contents {
		entry := metaFromMap(item, remotePath)
		itemPath := entry.Path
		if itemPath == "" && entry.Name != "" {
			itemPath = remotePath + "/" + entry.Name
			entry.Path = normRemotePath(itemPath)
		}
		if entry.IsFolder {
			if err := p.trashFolder(entry.Path); err != nil {
				return err
			}
		} else if err := p.deleteFileEntry(entry); err != nil {
			return err
		}
	}
	folderMeta, err := p.statRemote(remotePath)
	if err != nil {
		return err
	}
	return p.deleteFolderEntry(folderMeta)
}

func (p PCloud) Mkdir(path string) error {
	return p.ensurePath(p.pcloudPath(path))
}

func (p PCloud) Rm(path string) error {
	meta, err := p.resolveEntry(path)
	if err != nil {
		return err
	}
	if meta.IsFolder {
		return p.trashFolder(normRemotePath(strings.TrimSuffix(p.pcloudPath(path), "/")))
	}
	return p.deleteFileEntry(meta)
}

func (p PCloud) Mv(from string, to string) error {
	fromPath := p.pcloudPath(from)
	toPath := p.pcloudPath(to)
	if err := p.ensurePath(pathDir(toPath)); err != nil {
		return err
	}
	meta, err := p.resolveEntry(from)
	if err != nil {
		return err
	}
	method := "renamefile"
	if meta.IsFolder {
		method = "renamefolder"
	}
	res, err := p.get(method, map[string]string{
		"path":   normRemotePath(strings.TrimSuffix(fromPath, "/")),
		"topath": normRemotePath(strings.TrimSuffix(toPath, "/")),
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
	if r.Result != 0 {
		return NewError(r.Error, 400)
	}
	return nil
}

func (p PCloud) Touch(path string) error {
	return p.Save(path, strings.NewReader(""))
}

func (p PCloud) Save(path string, file io.Reader) error {
	remotePath := p.pcloudPath(path)
	parent := pathDir(remotePath)
	fname := pathBase(remotePath)
	if fname == "" {
		return NewError("invalid upload path", 400)
	}
	if err := p.ensurePath(parent); err != nil {
		return err
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", fname)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	q := url.Values{}
	q.Set("access_token", p.Bearer)
	q.Set("filename", fname)
	rawURL := p.apiURL("uploadfile") + "?" + q.Encode()
	rawURL += "&" + encodePathParam("path", parent)

	req, err := http.NewRequest("POST", rawURL, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
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
