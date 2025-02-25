package server_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/blang/semver"
	"github.com/reaper47/recipya/internal/app"
	"github.com/reaper47/recipya/internal/models"
)

func TestHandlers_General_Download(t *testing.T) {
	srv := newServerTest()

	uri := "/download"

	t.Run("must be logged in", func(t *testing.T) {
		assertMustBeLoggedIn(t, srv, http.MethodGet, uri+"/file.zip")
	})

	t.Run("file does not exist", func(t *testing.T) {
		srv.Files = &mockFiles{
			ReadTempFileFunc: func(name string) ([]byte, error) {
				return nil, errors.New("file does not exist")
			},
		}
		defer func() {
			srv.Files = &mockFiles{}
		}()

		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri+"/does-not-exists")

		assertStatus(t, rr.Code, http.StatusNotFound)
	})

	t.Run("file exists", func(t *testing.T) {
		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri+"/exists")

		assertStatus(t, rr.Code, http.StatusOK)
		assertHeader(t, rr, "Content-Type", "text/plain; charset=utf-8")
		assertHeader(t, rr, "Content-Disposition", `attachment; filename="exists"`)
		assertHeader(t, rr, "Content-Length", "6")
	})
}

func TestHandlers_General_Fetch(t *testing.T) {
	srv, ts, c := createWSServer()
	defer c.CloseNow()

	uri := ts.URL + "/fetch"

	t.Run("must be logged in", func(t *testing.T) {
		assertMustBeLoggedIn(t, srv, http.MethodGet, uri)
	})

	t.Run("no url to fetch", func(t *testing.T) {
		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri)

		assertStatus(t, rr.Code, http.StatusBadRequest)
		assertWebsocket(t, c, 1, `{"type":"toast","fileName":"","data":"","toast":{"action":"","background":"alert-error","message":"Invalid URL.","title":"General Error"}}`)
	})

	t.Run("malformed url", func(t *testing.T) {
		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri+"?url=slava-ukraini")

		assertStatus(t, rr.Code, http.StatusBadRequest)
		assertWebsocket(t, c, 1, `{"type":"toast","fileName":"","data":"","toast":{"action":"","background":"alert-error","message":"Invalid URL.","title":"General Error"}}`)
	})
}

func TestHandlers_General_Index(t *testing.T) {
	srv := newServerTest()
	srv.Repository = &mockRepository{
		UsersRegistered: []models.User{
			{ID: 1, Email: "test@example.com"},
		},
	}

	const uri = "/"

	t.Run("anonymous access", func(t *testing.T) {
		rr := sendRequestNoBody(srv, http.MethodGet, uri)

		assertStatus(t, rr.Code, http.StatusSeeOther)
		got := getBodyHTML(rr)
		want := []string{
			`<a href="/auth/login">See Other</a>`,
		}
		assertStringsInHTML(t, got, want)
		notWant := []string{
			`<span id="user-initials">A</span>`,
			"Add recipe",
			`<li id="recipes-sidebar-recipes" class="p-2 hover:bg-red-600 hover:text-white" hx-get="/recipes" hx-target="#content" hx-push-url="true" hx-swap-oob="true"> Recipes </li>`,
		}
		assertStringsNotInHTML(t, got, notWant)
	})

	t.Run("hide elements on main page when autologin enabled", func(t *testing.T) {
		app.Config.Server.IsAutologin = true
		defer func() {
			app.Config.Server.IsAutologin = false
		}()
		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri)
		got := getBodyHTML(rr)

		assertStatus(t, rr.Code, http.StatusOK)
		want := []string{
			`<li class="border-2 rounded-b-lg hover:bg-blue-100 dark:border-gray-500 dark:hover:bg-blue-600"><a hx-post="/auth/logout" class="flex" href="#"><svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 ml-0 self-center" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"/></svg><span class="pl-1 align-bottom">Log out</span></a></li>`,
		}
		assertStringsNotInHTML(t, got, want)
	})

	t.Run("logged in basic access", func(t *testing.T) {
		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri)
		got := getBodyHTML(rr)

		assertStatus(t, rr.Code, http.StatusOK)
		want := []string{
			`<title hx-swap-oob="true">Recipes | Recipya</title>`,
			`<button title="Open avatar menu" popovertarget="avatar_menu" popovertargetaction="toggle" class="" hx-get="/user-initials" hx-trigger="load" hx-target="#user-initials"><div tabindex="0" role="button" class="btn btn-ghost btn-circle avatar placeholder"><div class="bg-neutral text-neutral-content w-10 rounded-full"><span id="user-initials">A</span></div></div></button>`,
			`<div id="avatar_menu" popover style="inset: unset; top: 3.5rem; right: 0.5rem;" class="rounded-box z-10 shadow bg-base-200" _="on click if me.matches(':popover-open') then me.hidePopover()">`,
			`<div class="bg-neutral text-neutral-content w-10 rounded-full"><span id="user-initials">A</span></div>`,
			`<ul tabindex="0" class="menu">`,
			`<li onclick="document.activeElement?.blur()"><a href="/admin" hx-get="/admin" hx-target="#content" hx-push-url="true"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" d="M12 21v-8.25M15.75 21v-8.25M8.25 21v-8.25M3 9l9-6 9 6m-1.5 12V10.332A48.36 48.36 0 0 0 12 9.75c-2.551 0-5.056.2-7.5.582V21M3 21h18M12 6.75h.008v.008H12V6.75Z"></path></svg>Admin</a></li>`,
			`<li onclick="document.activeElement?.blur()"><a href="/reports" hx-get="/reports" hx-target="#content" hx-push-url="true"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" d="M3 3v1.5M3 21v-6m0 0 2.77-.693a9 9 0 0 1 6.208.682l.108.054a9 9 0 0 0 6.086.71l3.114-.732a48.524 48.524 0 0 1-.005-10.499l-3.11.732a9 9 0 0 1-6.085-.711l-.108-.054a9 9 0 0 0-6.208-.682L3 4.5M3 15V4.5"></path></svg>Reports</a></li><div class="divider m-0"></div>`,
			`<li onclick="document.activeElement?.blur()"><a href="https://recipya.musicavis.ca/docs" target="_blank"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6.042A8.967 8.967 0 0 0 6 3.75c-1.052 0-2.062.18-3 .512v14.25A8.987 8.987 0 0 1 6 18c2.305 0 4.408.867 6 2.292m0-14.25a8.966 8.966 0 0 1 6-2.292c1.052 0 2.062.18 3 .512v14.25A8.987 8.987 0 0 0 18 18a8.967 8.967 0 0 0-6 2.292m0-14.25v14.25"></path></svg>Guide</a></li>`,
			`<li class="cursor-pointer" onclick="settings_dialog.showModal()"><a hx-get="/settings" hx-target="#settings_dialog_content"><svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path> <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path></svg>Settings</a></li><div class="divider m-0"></div>`,
			`<li><a hx-post="/auth/logout"><svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5 ml-0 self-center" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"></path></svg>Log out</a></li></ul>`,
			`<div id="ws-notification-container" class="z-20 fixed bottom-0 right-0 p-6 cursor-default hidden"><div class="bg-blue-500 text-white px-4 py-2 rounded shadow-md"><p class="font-medium text-center pb-1"></p></div></div>`,
			"Add recipe",
			`<a class="tooltip tooltip-right active" data-tip="Recipes">`,
			`<a class="tooltip tooltip-right" data-tip="Cookbooks"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6">`,
			`<button id="add_recipe" class="btn btn-primary btn-sm hover:btn-accent" hx-get="/recipes/add" hx-target="#content" hx-trigger="mousedown" hx-push-url="true">Add recipe</button>`,
			`<button id="add_cookbook" class="btn btn-primary btn-sm hover:btn-accent" hx-post="/cookbooks" hx-prompt="Enter the name of your cookbook" hx-target="#cookbooks-display" hx-trigger="mousedown" hx-swap="beforeend">Add cookbook</button>`,
		}
		assertStringsInHTML(t, got, want)
		notWant := []string{`A powerful recipe manager that will blow your kitchen away`, `href="/auth/login"`}
		assertStringsNotInHTML(t, got, notWant)
	})

	t.Run("application update available", func(t *testing.T) {
		app.Info.IsUpdateAvailable = true
		defer func() {
			app.Info.IsUpdateAvailable = false
		}()

		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri)

		assertStatus(t, rr.Code, http.StatusOK)
		assertStringsInHTML(t, getBodyHTML(rr), []string{
			`<button title="Open avatar menu" popovertarget="avatar_menu" popovertargetaction="toggle" class="indicator" hx-get="/user-initials" hx-trigger="load" hx-target="#user-initials"><div tabindex="0" role="button" class="btn btn-ghost btn-circle avatar placeholder indicator"><span class="indicator-item indicator-start badge badge-sm badge-secondary z-30">New update</span><div class="bg-neutral text-neutral-content w-10 rounded-full"><span id="user-initials">A</span></div></div></button>`,
		})
	})
}

func TestHandlers_General_Placeholder(t *testing.T) {
	srv, ts, c := createWSServer()
	defer c.CloseNow()

	t.Run("must be logged in", func(t *testing.T) {
		assertMustBeLoggedIn(t, srv, http.MethodPost, ts.URL+"/placeholder")
		assertMustBeLoggedIn(t, srv, http.MethodPost, ts.URL+"/placeholder/restore")
	})
}

func TestHandlers_General_Update(t *testing.T) {
	srv, ts, c := createWSServer()
	defer c.CloseNow()

	originalFiles := srv.Files
	uri := ts.URL + "/update"

	t.Run("must be logged in", func(t *testing.T) {
		assertMustBeLoggedIn(t, srv, http.MethodGet, uri)
	})

	t.Run("error checking update", func(t *testing.T) {
		srv.Files = &mockFiles{
			updateAppFunc: func(_ semver.Version) error {
				return errors.New("beautiful death")
			},
		}
		defer func() {
			srv.Files = originalFiles
		}()

		rr := sendHxRequestAsLoggedInNoBody(srv, http.MethodGet, uri)

		assertStatus(t, rr.Code, http.StatusInternalServerError)
		assertWebsocket(t, c, 1, `{"type":"toast","fileName":"","data":"","toast":{"action":"","background":"alert-error","message":"Failed to update.","title":"General Error"}}`)
	})

	t.Run("no update available", func(t *testing.T) {
		srv.Files = &mockFiles{
			updateAppFunc: func(_ semver.Version) error {
				return app.ErrNoUpdate
			},
		}
		defer func() {
			srv.Files = originalFiles
		}()

		rr := sendHxRequestAsLoggedInNoBody(srv, http.MethodGet, uri)

		assertStatus(t, rr.Code, http.StatusNoContent)
		assertWebsocket(t, c, 1, `{"type":"toast","fileName":"","data":"","toast":{"action":"","background":"alert-warning","message":"No update available.","title":""}}`)
	})

	t.Run("update available", func(t *testing.T) {
		rr := sendHxRequestAsLoggedInNoBody(srv, http.MethodGet, uri)

		assertStatus(t, rr.Code, http.StatusNoContent)
		assertWebsocket(t, c, 1, `{"type":"toast","fileName":"","data":"","toast":{"action":"","background":"alert-info","message":"Application will reload in 5 seconds.","title":"Software updated"}}`)
	})
}

func TestHandlers_General_UserInitials(t *testing.T) {
	srv := newServerTest()
	srv.Repository = &mockRepository{
		UsersRegistered: []models.User{
			{ID: 1, Email: "test@example.com"},
		},
	}

	const uri = "/user-initials"

	t.Run("anonymous user doesn't have initials", func(t *testing.T) {
		assertMustBeLoggedIn(t, srv, http.MethodGet, uri)
	})

	t.Run("logged in user has initials", func(t *testing.T) {
		rr := sendRequestAsLoggedInNoBody(srv, http.MethodGet, uri)

		assertStatus(t, rr.Code, http.StatusOK)
		body := getBodyHTML(rr)
		want := string(strings.ToUpper(srv.Repository.Users()[0].Email)[0])
		if body != want {
			t.Fatalf("got %s but want %s", body, want)
		}
	})
}
