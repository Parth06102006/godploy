package gitroutes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Roshan-anand/godploy/internal/db"
	"github.com/Roshan-anand/godploy/internal/lib"
	"github.com/google/go-github/v84/github"
	"github.com/labstack/echo/v5"
)

type GitHubCreateAppRes struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	WebhookSecret string `json:"webhook_secret"`
	PEM           string `json:"pem"`
}

// TODO : replace localhost with config value actual URL

// get github app manifest data
func getManifestData() (string, error) {
	manifest := map[string]interface{}{
		"name": "GODPLOY",
		"url":  "http://localhost:8080",
		"hook_attributes": map[string]string{
			"url": "https://example.com/github/events", // TODO : replace with webhook endpoint URL
		},
		"redirect_url": "http://localhost:8080/api/provider/github/app/callback",
		// "callback_urls": []string{"http://localhost:8080/api/provider/github/app/callback"},
		"setup_url": "http://localhost:8080/api/provider/github/app/setup",
		"public":    true,
		"default_permissions": map[string]string{
			"contents": "read",
			"metadata": "read",
		},
		"default_events": []string{"push"},
	}

	manifestDataB, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}

	return string(manifestDataB), nil
}

// auto-submitting form template — POST to GitHub with manifest in body (required by GitHub manifest flow)
const githubManifestFormTmpl = `<!DOCTYPE html>
<html>
<body>
  <form id="mf" action="https://github.com/settings/apps/new?state={{.State}}" method="POST">
    <input type="hidden" name="manifest" value="{{.Manifest}}">
  </form>
  <script>document.getElementById("mf").submit();</script>
</body>
</html>`

// initiate github app creation
//
// route: GET /api/provider/github/app/create
func (h *GitHandler) CreateGithubApp(c *echo.Context) error {
	q := h.Server.DB.Queries
	u := c.Get(h.Server.Config.EchoCtxUserKey).(lib.AuthUser)

	state, err := lib.GenerateCSRFToken()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to create github app"})
	}

	user, err := q.GetUserByEmail(h.qCtx, u.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to create github app"})
	}

	if err := q.CreateRedirectSession(h.qCtx, db.CreateRedirectSessionParams{
		State:     state,
		OrgID:     user.CurrentOrgID,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to create github app"})
	}

	manifest, err := getManifestData()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to create github app"})
	}

	tmpl, err := template.New("manifest").Parse(githubManifestFormTmpl)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to create github app"})
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, map[string]string{
		"State":    state,
		"Manifest": manifest,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to create github app"})
	}

	return c.HTML(http.StatusOK, buf.String())
}

// get github app credentials from GitHub
//
// route: GET /api/provider/github/app/callback
func (h *GitHandler) CreateGithubAppCallback(c *echo.Context) error {
	query := h.Server.DB.Queries
	// u := c.Get(h.Server.Config.EchoCtxUserKey).(lib.AuthUser)

	code := c.QueryParam("code")
	state := c.QueryParam("state")

	// validate the state
	sData, err := query.GetRedirectSession(h.qCtx, state)
	if err != nil {
		return c.JSON(http.StatusBadRequest, lib.Res{Message: "Invalid state"})
	}

	if time.Now().After(sData.ExpiresAt) {
		go removeSession(query, state)
		return c.JSON(http.StatusBadRequest, lib.Res{Message: "State has expired"})
	}

	conversionURL := fmt.Sprintf("https://api.github.com/app-manifests/%s/conversions", code)
	req, err := http.NewRequest("POST", conversionURL, nil)
	if err != nil {
		return c.Redirect(http.StatusFound, "/?github_error=internal")
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return c.Redirect(http.StatusFound, "/?github_error=github_api_error")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return c.Redirect(http.StatusFound, "/?github_error=code_invalid")
	}

	var convRes GitHubCreateAppRes
	if err := json.NewDecoder(resp.Body).Decode(&convRes); err != nil {
		return c.Redirect(http.StatusFound, "/?github_error=github_api_error")
	}

	// encrypt PEM
	encryptedPem, err := lib.EncryptPEM(convRes.PEM)
	if err != nil {
		return c.Redirect(http.StatusFound, "/?github_error=internal")
	}

	// store the app credentials in db
	if err := query.CreateGithubApp(h.qCtx, db.CreateGithubAppParams{
		ID:             lib.NewID(),
		AppID:          convRes.ID,
		OrganizationID: sData.OrgID,
		WebhookSecret:  convRes.WebhookSecret,
		PemKey:         encryptedPem,
	}); err != nil {
		return c.Redirect(http.StatusFound, "/?github_error=internal")
	}

	go removeSession(query, state)

	installUrl := fmt.Sprintf("https://github.com/apps/%s/installations/new", convRes.Slug)
	return c.Redirect(http.StatusFound, installUrl)
}

// installing github app
//
// route: GET /api/provider/github/app/setup
func (h *GitHandler) SetupGithubApp(c *echo.Context) error {
	query := h.Server.DB.Queries
	u := c.Get(h.Server.Config.EchoCtxUserKey).(lib.AuthUser)

	instllation_id, err := strconv.ParseInt(c.QueryParam("installation_id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, lib.Res{Message: "Invalid installation ID"})
	}

	user, err := query.GetUserByEmail(h.qCtx, u.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to setup github app"})
	}

	// varify installation ID
	ghApp, err := query.GetGithubApp(h.qCtx, user.CurrentOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to setup github app"})
	}

	pem, err := lib.DecryptPEM(ghApp.PemKey)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to setup github app"})
	}

	// use app-level client (JWT auth) — GetInstallation requires JWT, not installation token
	appClient, err := lib.CreateAppClient(ghApp.AppID, []byte(pem))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to setup github app"})
	}

	// verify installation ID by making an authenticated request to GitHub API
	_, _, err = appClient.Apps.GetInstallation(context.Background(), instllation_id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, lib.Res{Message: "Invalid installation ID"})
	}

	if err := query.InsertInstallationID(h.qCtx, db.InsertInstallationIDParams{
		InstallationID: sql.NullInt64{
			Int64: instllation_id,
			Valid: true,
		},
		OrganizationID: user.CurrentOrgID,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to setup github app"})
	}

	return c.Redirect(http.StatusFound, "http://localhost:8080/#/")
}

// installing github app
//
// route: GET /api/provider/github/repo/list
func (h *GitHandler) GetGithubRepoList(c *echo.Context) error {
	query := h.Server.DB.Queries
	u := c.Get(h.Server.Config.EchoCtxUserKey).(lib.AuthUser)

	user, err := query.GetUserByEmail(h.qCtx, u.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to get github repos"})
	}

	ghApp, err := query.GetGithubApp(h.qCtx, user.CurrentOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to get github repos"})
	}

	pem, err := lib.DecryptPEM(ghApp.PemKey)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to get github repos"})
	}

	ghClient, err := lib.CreateGithubClient(context.Background(), ghApp.AppID, ghApp.InstallationID.Int64, []byte(pem))
	if err != nil {
		fmt.Println("create gh client error :", err)
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to get github repos"})
	}

	repos, res, err := ghClient.Apps.ListRepos(h.ghCtx, &github.ListOptions{
		Page: 1,
	})
	if err != nil {
		fmt.Printf("GitHub API error: %v\nResponse: %v\n", err, res)
		return c.JSON(http.StatusInternalServerError, lib.Res{Message: "Failed to get github repos"})
	}

	return c.JSON(http.StatusOK, repos)
}
