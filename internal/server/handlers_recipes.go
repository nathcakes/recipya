package server

import (
	"bytes"
	"fmt"
	"github.com/google/uuid"
	"github.com/reaper47/recipya/internal/app"
	"github.com/reaper47/recipya/internal/models"
	"github.com/reaper47/recipya/internal/templates"
	"github.com/reaper47/recipya/internal/utils/extensions"
	"github.com/reaper47/recipya/web/components"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

func (s *Server) recipesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageNumber, err := strconv.ParseUint(r.URL.Query().Get("page"), 10, 64)
		if err != nil {
			pageNumber = 1
		}

		sorts := r.URL.Query().Get("sort")
		if sorts == "" {
			sorts = "default"
		}

		mode := r.URL.Query().Get("mode")
		if mode == "" {
			mode = "full"
		}

		userID := getUserID(r)

		p, err := newRecipesPagination(s, userID, pageNumber, sorts, false)
		if err != nil {
			s.Brokers.SendToast(models.NewErrorGeneralToast("Error updating pagination."), userID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_ = components.RecipesIndex(templates.Data{
			About:           templates.NewAboutData(),
			Functions:       templates.NewFunctionsData[int64](),
			IsAdmin:         userID == 1,
			IsAutologin:     app.Config.Server.IsAutologin,
			IsAuthenticated: true,
			IsHxRequest:     r.Header.Get("HX-Request") == "true",
			Pagination:      p,
			Recipes:         s.Repository.Recipes(userID, pageNumber, sorts),
			Searchbar: templates.SearchbarData{
				Mode: mode,
				Sort: sorts,
			},
		}).Render(r.Context(), w)
	}
}

func newRecipesPagination(srv *Server, userID int64, page uint64, sorts string, isSwap bool) (templates.Pagination, error) {
	counts, err := srv.Repository.Counts(userID)
	if err != nil {
		return templates.Pagination{}, err
	}

	numPages := counts.Recipes / templates.ResultsPerPage
	if numPages == 0 {
		numPages = 1
	}

	htmx := templates.PaginationHtmx{
		IsSwap: isSwap,
		Target: "#content",
	}
	return templates.NewPagination(page, numPages, counts.Recipes, templates.ResultsPerPage, "/recipes", "sort="+sorts, htmx), nil
}

func recipesAddHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isHxRequest := r.Header.Get("Hx-Request") == "true"
		if isHxRequest {
			parsedURL, err := url.Parse(r.Header.Get("HX-Current-Url"))
			if err == nil && parsedURL.Path == "/recipes/add/unsupported-website" {
				w.Header().Set("HX-Trigger", models.NewInfoToast("", "Website requested.", "").Render())
			}
		}

		_ = components.AddRecipe(templates.Data{
			About:           templates.NewAboutData(),
			IsAdmin:         getUserID(r) == 1,
			IsAuthenticated: true,
			IsHxRequest:     isHxRequest,
		}).Render(r.Context(), w)
	}
}

func (s *Server) recipesAddImportHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const maxSize = 512 << 20
		r.Body = http.MaxBytesReader(w, r.Body, maxSize)

		userID := getUserID(r)
		userIDAttr := slog.Int64("userID", userID)

		if !s.Brokers.Has(userID) {
			w.Header().Set("HX-Trigger", models.NewWarningWSToast("Connection lost. Please reload page.").Render())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		s.Brokers.SendProgressStatus("Preparing...", true, 0, -1, userID)

		err := r.ParseMultipartForm(maxSize)
		if err != nil {
			s.Brokers.HideNotification(userID)
			msg := "Could not parse the uploaded files."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		files, filesOk := r.MultipartForm.File["files"]
		if !filesOk {
			s.Brokers.HideNotification(userID)
			msg := "Could not retrieve the files or the directory from the form."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		go func() {
			var (
				progress  = make(chan models.Progress)
				recipes   = s.Files.ExtractRecipes(files)
				report    = models.NewReport(models.ImportReportType)
				total     = len(recipes)
				recipeIDs []int64
			)

			if total == 0 {
				s.Brokers.HideNotification(userID)
				s.Brokers.SendToast(models.NewWarningToast("No recipes found.", "", ""), userID)
				return
			}

			s.Brokers.SendProgress(fmt.Sprintf("Importing 1/%d", total), 1, total, userID)
			now := time.Now()

			go func() {
				defer close(progress)

				recipeIDs, report.Logs, err = s.Repository.AddRecipes(recipes, userID, progress)
				if err != nil {
					slog.Error("Error adding recipes", userIDAttr, "recipes", recipes, "error", err)
				}
			}()

			for p := range progress {
				s.Brokers.SendProgress(fmt.Sprintf("Importing %d/%d", p.Value, p.Total), p.Value, p.Total, userID)
			}

			report.ExecTime = time.Since(now)
			s.Repository.AddReport(report, userID)
			s.Brokers.HideNotification(userID)

			numSuccess := len(recipeIDs)
			skipped := total - numSuccess

			redirect := "/reports?view=latest"
			if numSuccess == 1 {
				redirect = "/recipes/" + strconv.FormatInt(recipeIDs[0], 10)
			}

			slog.Info("Imported recipes", userIDAttr, "imported", numSuccess, "skipped", skipped, "total", total)
			s.Brokers.SendToast(models.NewInfoToast("Operation Successful", fmt.Sprintf("Imported %d recipes. %d skipped", numSuccess, skipped), "View "+redirect), userID)
		}()

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *Server) recipeAddManualHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		categories, err := s.Repository.Categories(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_ = components.AddRecipeManual(templates.Data{
			About:           templates.NewAboutData(),
			IsAdmin:         userID == 1,
			IsAuthenticated: true,
			IsHxRequest:     r.Header.Get("Hx-Request") == "true",
			View:            &templates.ViewRecipeData{Categories: categories},
		}).Render(r.Context(), w)
	}
}

func (s *Server) recipeAddManualPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 128<<20)

		userID := getUserID(r)
		userIDAttr := slog.Int64("userID", userID)

		err := r.ParseMultipartForm(128 << 20)
		if err != nil {
			msg := "Could not parse the form."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var imageUUIDs []uuid.UUID
		imageFiles, ok := r.MultipartForm.File["images"]
		if ok {
			for _, file := range imageFiles {
				fileAttr := slog.String("file", file.Filename)

				f, err := file.Open()
				if err != nil {
					slog.Error("Could not open the image from the form.", userIDAttr, fileAttr, "error", err)
					continue
				}

				imageUUID, err := s.Files.UploadImage(f)
				if err != nil {
					_ = f.Close()
					slog.Error("Error uploading image.", userIDAttr, fileAttr, "error", err)
					continue
				}
				imageUUIDs = append(imageUUIDs, imageUUID)

				_ = f.Close()
			}
		}

		ingredients := make([]string, 0)
		i := 1
		for {
			key := fmt.Sprintf("ingredient-%d", i)
			if r.Form.Has(key) {
				ingredients = append(ingredients, r.FormValue(key))
				i++
			} else {
				break
			}
		}

		instructions := make([]string, 0)
		i = 1
		for {
			key := fmt.Sprintf("instruction-%d", i)
			if r.Form.Has(key) {
				instructions = append(instructions, r.FormValue(key))
				i++
			} else {
				break
			}
		}

		times, err := models.NewTimes(r.FormValue("time-preparation"), r.FormValue("time-cooking"))
		if err != nil {
			msg := "Error parsing times."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		yield, err := strconv.ParseInt(r.FormValue("yield"), 10, 16)
		if err != nil {
			msg := "Error parsing yield."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		recipe := models.Recipe{
			Category:     strings.ToLower(r.FormValue("category")),
			CreatedAt:    time.Time{},
			Cuisine:      "",
			Description:  r.FormValue("description"),
			Images:       imageUUIDs,
			Ingredients:  ingredients,
			Instructions: instructions,
			Keywords:     nil,
			Name:         r.FormValue("title"),
			Nutrition: models.Nutrition{
				Calories:           r.FormValue("calories"),
				Cholesterol:        r.FormValue("cholesterol"),
				Fiber:              r.FormValue("fiber"),
				Protein:            r.FormValue("protein"),
				SaturatedFat:       r.FormValue("saturated-fat"),
				Sodium:             r.FormValue("sodium"),
				Sugars:             r.FormValue("sugars"),
				TotalCarbohydrates: r.FormValue("total-carbohydrates"),
				TotalFat:           r.FormValue("total-fat"),
				UnsaturatedFat:     "",
			},
			Times:     times,
			Tools:     nil,
			UpdatedAt: time.Time{},
			URL:       r.FormValue("source"),
			Yield:     int16(yield),
		}

		recipeIDs, _, err := s.Repository.AddRecipes(models.Recipes{recipe}, userID, nil)
		if err != nil {
			msg := "Could not add recipe."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorDBToast(msg), userID)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		slog.Info("Recipe added", userIDAttr, "recipeNumber", recipeIDs[0], "recipe", recipe)
		w.Header().Set("HX-Redirect", "/recipes/"+strconv.FormatInt(recipeIDs[0], 10))
		w.WriteHeader(http.StatusCreated)
	}
}

func recipeAddManualIngredientHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			w.Header().Set("HX-Trigger", models.NewErrorFormToast("Could not parse the form.").Render())
			w.WriteHeader(http.StatusNoContent)
			return
		}

		i := 1
		for {
			if !r.Form.Has("ingredient-" + strconv.Itoa(i)) {
				break
			}
			i++
		}

		if r.Form.Get(fmt.Sprintf("ingredient-%d", i-1)) == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		_ = components.AddIngredient(i).Render(r.Context(), w)
	}
}

func recipeAddManualIngredientDeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		form := r.Form
		if err != nil || form == nil {
			w.Header().Set("HX-Trigger", models.NewErrorFormToast("Could not parse the form.").Render())
			w.WriteHeader(http.StatusNoContent)
			return
		}

		entry, err := parsePathPositiveID(r.PathValue("entry"))
		if err != nil {
			w.Header().Set("HX-Trigger", models.NewErrorReqToast("Ingredient entry must be >= 1.").Render())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		count := 0
		i := int64(1)
		for {
			if !form.Has(fmt.Sprintf("ingredient-%d", i)) {
				break
			}

			i++
			count++
		}

		if count == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		var sb strings.Builder

		curr := 1
		i = 1
		for {
			key := fmt.Sprintf("ingredient-%d", i)
			if !form.Has(key) {
				break
			}

			if entry == i {
				i++
				continue
			}

			currStr := strconv.Itoa(curr)
			xs := []string{
				`<li class="pb-2"><div class="grid grid-flow-col items-center">`,
				`<label><input required type="text" name="ingredient-` + currStr + `" placeholder="Ingredient #` + currStr + `" class="input input-bordered input-sm w-full" ` + `value="` + form.Get(key) + `" _="on keydown if event.key is 'Enter' then halt the event then get next <button/> from the parentElement of me then call htmx.trigger(it, 'click')"></label>`,
				`<div class="ml-2">&nbsp;<button type="button" class="btn btn-square btn-sm btn-outline btn-success" title="Shortcut: Enter" hx-post="/recipes/add/manual/ingredient" hx-target="#ingredients-list" hx-swap="beforeend" hx-include="[name^='ingredient']">+</button>`,
				`&nbsp;<button type="button" class="delete-button btn btn-square btn-sm btn-outline btn-error" hx-target="#ingredients-list" hx-post="/recipes/add/manual/ingredient/` + currStr + `" hx-include="[name^='ingredient']">-</button>`,
				`&nbsp;<div class="inline-block h-4 cursor-move handle"><svg xmlns="http://www.w3.org/2000/svg" class="md:w-4 md:h-4 w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4"/></svg></div>`,
				`</div></div></li>`,
			}
			for _, x := range xs {
				sb.WriteString(x)
			}

			i++
			curr++
		}

		_, _ = fmt.Fprint(w, sb.String())
	}
}

func recipeAddManualInstructionHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		form := r.Form
		if err != nil || form == nil {
			w.Header().Set("HX-Trigger", models.NewErrorFormToast("Could not parse the form.").Render())
			w.WriteHeader(http.StatusNoContent)
			return
		}

		i := 1
		for {
			if !form.Has("instruction-" + strconv.Itoa(i)) {
				break
			}

			i++
		}

		if form.Get(fmt.Sprintf("instruction-%d", i-1)) == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		_ = components.AddInstruction(i).Render(r.Context(), w)
	}
}

func recipeAddManualInstructionDeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		form := r.Form
		if err != nil || form == nil {
			w.Header().Set("HX-Trigger", models.NewErrorFormToast("Could not parse the form.").Render())
			w.WriteHeader(http.StatusNoContent)
			return
		}

		entry, err := parsePathPositiveID(r.PathValue("entry"))
		if err != nil {
			w.Header().Set("HX-Trigger", models.NewErrorReqToast("Ingredient entry must be >= 1.").Render())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		count := 0
		i := int64(1)
		for {
			if !form.Has(fmt.Sprintf("instruction-%d", i)) {
				break
			}

			i++
			count++
		}

		if count == 1 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}

		var sb strings.Builder

		curr := 1
		i = 1
		for {
			key := fmt.Sprintf("instruction-%d", i)
			if !form.Has(key) {
				break
			}

			if entry == i {
				i++
				continue
			}

			currStr := strconv.Itoa(curr)
			value := form.Get(key)

			xs := []string{
				`<li class="pt-2 md:pl-0"><div class="flex">`,
				`<label class="w-11/12"><textarea required name="instruction-` + currStr + `" rows="3" class="textarea textarea-bordered w-full" placeholder="Instruction #` + currStr + `" _="on keydown if event.ctrlKey and event.key === 'Enter' then halt the event then get next <div/> from the parentElement of me then get first <button/> in it then call htmx.trigger(it, 'click')">` + value + `</textarea>&nbsp;</label>`,
				`<div class="grid ml-2">`,
				`<button type="button" class="btn btn-square btn-sm btn-outline btn-success" title="Shortcut: CTRL + Enter" hx-post="/recipes/add/manual/instruction" hx-target="#instructions-list" hx-swap="beforeend" hx-include="[name^='instruction']">+</button>`,
				`<button type="button" class="delete-button btn btn-square btn-sm btn-outline btn-error" hx-target="#instructions-list" hx-post="/recipes/add/manual/instruction/` + currStr + `" hx-include="[name^='instruction']">-</button>`,
				`<div class="h-4 cursor-move handle grid place-content-center"><svg xmlns="http://www.w3.org/2000/svg" class="md:w-4 md:h-4 w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4"/></svg></div>`,
				`</div></div></li>`,
			}
			for _, x := range xs {
				sb.WriteString(x)
			}

			i++
			curr++
		}

		_, _ = fmt.Fprint(w, sb.String())
	}
}

func (s *Server) recipesAddOCRHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		userIDAttr := slog.Int64("userID", userID)

		if app.Config.Integrations.AzureDI.Key == "" || app.Config.Integrations.AzureDI.Endpoint == "" {
			s.Brokers.SendToast(models.NewWarningToast("Feature Disabled", "Please consult the docs to enable OCR.", ""), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// 1. Retrieve the files from the body.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<24)

		err := r.ParseMultipartForm(1 << 24)
		if err != nil {
			msg := "Could not parse the form."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		files, ok := r.MultipartForm.File["files"]
		if !ok {
			msg := "Could not retrieve the image from the form."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// 2. Filter the files.
		var (
			validImageFormats = []string{".jpg", ".jpeg", ".png", ".bmp", ".tiff", ".heif"}
			validDocFormats   = []string{".pdf"}
		)

		files = slices.DeleteFunc(files, func(fh *multipart.FileHeader) bool {
			return !slices.Contains(slices.Concat(validImageFormats, validDocFormats), filepath.Ext(fh.Filename))
		})

		if len(files) == 0 {
			msg := "No valid files found in request."
			slog.Error(msg, userIDAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorFormToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// 3. Compartmentalize the files.
		var (
			imageFiles []*multipart.FileHeader
			docFiles   []io.Reader
		)

		for _, fh := range files {
			ext := filepath.Ext(fh.Filename)
			if slices.Contains(validImageFormats, ext) {
				imageFiles = append(imageFiles, fh)
			} else if slices.Contains(validDocFormats, ext) {
				open, err := fh.Open()
				if err != nil {
					continue
				}

				xb, err := io.ReadAll(open)
				if err != nil {
					continue
				}

				docFiles = append(docFiles, bytes.NewBuffer(xb))
			}
		}

		// 4. Merge images to one PDF file, if applicable.
		if len(imageFiles) > 1 {
			openImages := make([]io.Reader, 0, len(imageFiles))
			for _, file := range imageFiles[1:] {
				f, err := file.Open()
				if err != nil {
					continue
				}

				buf := bytes.NewBuffer(nil)
				_, err = io.Copy(buf, f)
				if err != nil {
					_ = f.Close()
					continue
				}
				_ = f.Close()

				openImages = append(openImages, buf)
			}

			imagePDF := s.Files.MergeImagesToPDF(openImages)
			if imagePDF != nil {
				docFiles = append(docFiles, imagePDF)
			}
		} else if len(imageFiles) == 1 {
			f, err := imageFiles[0].Open()
			if err == nil {
				buf := bytes.NewBuffer(nil)
				_, err = io.Copy(buf, f)
				if err == nil {
					docFiles = append(docFiles, buf)
				}
				_ = f.Close()
			}
		}

		// 5. Process the files.
		go func(id int64, data []io.Reader) {
			idAttr := slog.Int64("id", id)
			s.Brokers.SendProgressStatus("Analyzing...", true, 0, -1, id)

			recipes, err := s.Integrations.ProcessImageOCR(data)
			if err != nil {
				msg := "Could not process OCR."
				slog.Error(msg, idAttr, "error", err)
				s.Brokers.HideNotification(id)
				s.Brokers.SendToast(models.NewErrorToast("Integrations Error", msg, ""), id)
				return
			}

			recipeIDs, _, err := s.Repository.AddRecipes(recipes, id, nil)
			if err != nil {
				s.Brokers.HideNotification(id)
				s.Brokers.SendToast(models.NewErrorDBToast("Recipes could not be added."), id)
				return
			}

			s.Brokers.HideNotification(id)

			switch len(recipeIDs) {
			case 0:
				slog.Error("No recipes saved in database.")
				s.Brokers.SendToast(models.NewErrorReqToast("Failed to process. Please check logs."), id)

			case 1:
				msg := "Recipe scanned and uploaded."
				slog.Info(msg, "id", recipeIDs[0])
				s.Brokers.SendToast(models.NewInfoToast("Operation Successful", msg, fmt.Sprintf("View /recipes/%d", recipeIDs[0])), id)
			default:
				msg := "Recipes scanned and uploaded."
				slog.Info(msg, "ids", recipeIDs)
				s.Brokers.SendToast(models.NewInfoToast("Operation Successful", msg, ""), id)
			}
		}(userID, docFiles)

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *Server) recipesAddWebsiteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		userIDAttr := slog.Int64("userID", userID)

		if !s.Brokers.Has(userID) {
			w.Header().Set("HX-Trigger", models.NewWarningWSToast("Connection lost. Please reload page.").Render())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		urls := slices.DeleteFunc(strings.Split(r.FormValue("urls"), "\n"), func(s string) bool {
			return strings.TrimSpace(s) == ""
		})
		for i, u := range urls {
			urls[i] = strings.TrimSpace(u)
		}

		if len(urls) == 0 {
			slog.Error("No urls", userIDAttr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		urls = extensions.Unique(urls)

		invalid := make(map[int]struct{})
		for i, rawURL := range urls {
			_, err := url.ParseRequestURI(rawURL)
			if err != nil {
				invalid[i] = struct{}{}
				continue
			}
		}

		validURLs := make([]string, 0, len(urls)-len(invalid))
		for i, rawURL := range urls {
			_, ok := invalid[i]
			if !ok {
				validURLs = append(validURLs, rawURL)
			}
		}

		if len(validURLs) == 0 {
			msg := "No valid URLs found."
			slog.Error(msg, userIDAttr, "urls", urls)
			s.Brokers.SendToast(models.NewErrorReqToast(msg), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		go func() {
			var (
				count     atomic.Int64
				processed int
				progress  = make(chan models.Progress)
				report    = models.NewReport(models.ImportReportType)
				total     = len(validURLs)
				recipeIDs = make([]int64, 0, total)
			)

			s.Brokers.SendProgress(fmt.Sprintf("Fetching 1/%d", total), 1, total, userID)

			now := time.Now()

			go func() {
				for _, rawURL := range validURLs {
					go func(u string) {
						defer func() {
							progress <- models.Progress{Total: total}
						}()

						rs, err := s.Scraper.Scrape(u, s.Files)
						if err != nil {
							report.Logs = append(report.Logs, models.ReportLog{Error: err.Error(), Title: u})
							return
						}

						recipe, err := rs.Recipe()
						if err != nil {
							report.Logs = append(report.Logs, models.ReportLog{Error: err.Error(), Title: u})
							return
						}

						ids, _, err := s.Repository.AddRecipes(models.Recipes{*recipe}, userID, nil)
						if err != nil {
							report.Logs = append(report.Logs, models.ReportLog{Error: err.Error(), Title: u})
							return
						}

						report.Logs = append(report.Logs, models.ReportLog{IsSuccess: true, Title: u})
						recipeIDs = append(recipeIDs, ids[0])

						count.Add(1)
					}(rawURL)
				}
			}()

			for p := range progress {
				processed++
				s.Brokers.SendProgress(fmt.Sprintf("Fetching %d/%d", processed, p.Total), processed, p.Total, userID)
				if processed == total {
					close(progress)
				}
			}

			report.ExecTime = time.Since(now)

			s.Repository.AddReport(report, userID)
			s.Brokers.HideNotification(userID)

			var (
				toast      models.Toast
				numSuccess = count.Load()
			)

			if numSuccess == 0 && total == 1 {
				msg := "Fetching the recipe failed."
				toast = models.NewErrorToast("Operation Failed", msg, "View /reports?view=latest")
				slog.Error(msg, userIDAttr)
			} else if numSuccess == 1 && total == 1 {
				msg := "Recipe has been added to your collection."
				toast = models.NewInfoToast("Operation Successful", msg, fmt.Sprintf("View /recipes/%d", recipeIDs[0]))
				slog.Info(msg, userIDAttr, "recipeID", recipeIDs[0])
			} else {
				skipped := int64(total) - numSuccess
				toast = models.NewInfoToast("Operation Successful", fmt.Sprintf("Fetched %d recipes. %d skipped", numSuccess, skipped), "View /reports?view=latest")
				slog.Info("Fetched recipes", userIDAttr, "recipes", recipeIDs, "fetched", numSuccess, "skipped", skipped, "total", total)
			}

			s.Brokers.SendToast(toast, userID)
		}()

		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *Server) recipeDeleteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		userIDAttr := slog.Int64("userID", userID)

		id, err := parsePathPositiveID(r.PathValue("id"))
		if err != nil {
			slog.Error("Failed to parse id", userIDAttr, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		idAttr := slog.Int64("id", id)

		err = s.Repository.DeleteRecipe(id, userID)
		if err != nil {
			msg := "Recipe could not be deleted."
			slog.Error(msg, userIDAttr, idAttr, "error", err)
			s.Brokers.SendToast(models.NewErrorDBToast(msg), userID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		slog.Info("Recipe deleted", userIDAttr, idAttr)
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) recipesEditHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePathPositiveID(r.PathValue("id"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		userID := getUserID(r)
		recipe, err := s.Repository.Recipe(id, userID)
		if err != nil {
			s.Brokers.SendToast(models.NewErrorDBToast("Failed to retrieve recipe."), userID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		_ = components.EditRecipe(templates.Data{
			About:           templates.NewAboutData(),
			IsAdmin:         userID == 1,
			IsAuthenticated: true,
			IsHxRequest:     r.Header.Get("HX-Request") == "true",
			View:            templates.NewViewRecipeData(id, recipe, true, false),
		}).Render(r.Context(), w)
	}
}

func (s *Server) recipesEditPostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		userIDAttr := slog.Int64("userID", userID)
		r.Body = http.MaxBytesReader(w, r.Body, 128<<20)

		err := r.ParseMultipartForm(128 << 20)
		if err != nil {
			s.Brokers.SendToast(models.NewErrorFormToast("Could not parse the form."), userID)
			w.Header().Set("HX-Trigger", models.NewErrorFormToast("Could not parse the form.").Render())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		updatedRecipe := models.Recipe{
			Category:     r.FormValue("category"),
			Description:  r.FormValue("description"),
			Ingredients:  make([]string, 0),
			Instructions: make([]string, 0),
			Name:         r.FormValue("title"),
			Nutrition: models.Nutrition{
				Calories:           r.FormValue("calories"),
				Cholesterol:        r.FormValue("cholesterol"),
				Fiber:              r.FormValue("fiber"),
				Protein:            r.FormValue("protein"),
				SaturatedFat:       r.FormValue("saturated-fat"),
				Sodium:             r.FormValue("sodium"),
				Sugars:             r.FormValue("sugars"),
				TotalCarbohydrates: r.FormValue("total-carbohydrates"),
				TotalFat:           r.FormValue("total-fat"),
			},
			URL: r.FormValue("source"),
		}

		recipeNumStr := r.PathValue("id")
		recipeNumAttr := slog.String("recipeID", recipeNumStr)

		recipeNum, err := parsePathPositiveID(recipeNumStr)
		if err != nil {
			slog.Error("Failed to parse id", userIDAttr, recipeNumAttr, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		imageFiles, ok := r.MultipartForm.File["images"]
		if ok {
			for _, imageFile := range imageFiles {
				file, err := imageFile.Open()
				if err != nil {
					msg := "Could not open the image from the form."
					slog.Error(msg, userIDAttr, recipeNumAttr, "error", err)
					s.Brokers.SendToast(models.NewErrorGeneralToast(msg), userID)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				imageUUID, err := s.Files.UploadImage(file)
				if err != nil {
					_ = file.Close()
					msg := "Error uploading image."
					slog.Error(msg, userIDAttr, "error", err)
					s.Brokers.SendToast(models.NewErrorGeneralToast(msg), userID)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				_ = file.Close()

				if imageUUID != uuid.Nil {
					updatedRecipe.Images = append(updatedRecipe.Images, imageUUID)
				}
			}
		}

		times, err := models.NewTimes(r.FormValue("time-preparation"), r.FormValue("time-cooking"))
		if err == nil {
			updatedRecipe.Times = times
		}

		i := 1
		for {
			ing := "ingredient-" + strconv.Itoa(i)
			if !r.Form.Has(ing) {
				break
			}
			updatedRecipe.Ingredients = append(updatedRecipe.Ingredients, r.FormValue(ing))
			i++
		}

		i = 1
		for {
			ing := "instruction-" + strconv.Itoa(i)
			if !r.Form.Has(ing) {
				break
			}
			updatedRecipe.Instructions = append(updatedRecipe.Instructions, r.FormValue(ing))
			i++
		}

		yield, err := strconv.ParseInt(r.FormValue("yield"), 10, 16)
		if err == nil {
			updatedRecipe.Yield = int16(yield)
		}

		err = s.Repository.UpdateRecipe(&updatedRecipe, userID, recipeNum)
		if err != nil {
			msg := "Error updating recipe."
			slog.Error(msg, userIDAttr, "updatedRecipe", updatedRecipe, "error", err)
			s.Brokers.SendToast(models.NewErrorGeneralToast(msg), userID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		slog.Info("Recipe updated", userIDAttr, "recipeNum", recipeNumStr, "updatedRecipe", updatedRecipe)
		w.Header().Set("HX-Redirect", "/recipes/"+recipeNumStr)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) recipeShareHandler(w http.ResponseWriter, r *http.Request) {
	_, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		s.recipesViewShareHandler().ServeHTTP(w, r)
		return
	}

	userID, isLoggedIn := s.findUserID(r)

	share, err := s.Repository.RecipeShared(r.URL.String())
	if err != nil {
		notFoundHandler(w, r)
		return
	}

	recipe, err := s.Repository.Recipe(share.RecipeID, share.UserID)
	if err != nil {
		notFoundHandler(w, r)
		return
	}

	_ = components.ViewRecipe(templates.Data{
		About:           templates.NewAboutData(),
		IsAdmin:         userID == 1,
		IsAuthenticated: isLoggedIn,
		IsHxRequest:     r.Header.Get("Hx-Request") == "true",
		View:            templates.NewViewRecipeData(share.RecipeID, recipe, userID == share.UserID, true),
	}).Render(r.Context(), w)
}

func (s *Server) recipeScaleHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)

		yield, err := strconv.ParseInt(r.URL.Query().Get("yield"), 10, 16)
		if err != nil {
			s.Brokers.SendToast(models.NewErrorGeneralToast("No yield in the query."), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if yield <= 0 {
			s.Brokers.SendToast(models.NewErrorGeneralToast("Yield must be greater than zero."), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		id, err := parsePathPositiveID(r.PathValue("id"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		recipe, err := s.Repository.Recipe(id, userID)
		if err != nil {
			s.Brokers.SendToast(models.NewErrorGeneralToast("Recipe not found."), userID)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		recipe.Scale(int16(yield))

		_ = components.IngredientsInstructions(&templates.ViewRecipeData{Recipe: recipe}).Render(r.Context(), w)
	}
}

func (s *Server) recipeSharePostHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		userIDAttr := slog.Int64("userID", userID)

		recipeID, err := parsePathPositiveID(r.PathValue("id"))
		if err != nil {
			slog.Error("Failed to parse recipe ID", userIDAttr, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		share := models.Share{CookbookID: -1, RecipeID: recipeID, UserID: userID}

		link, err := s.Repository.AddShareLink(share)
		if err != nil {
			msg := "Failed to create share link."
			slog.Error(msg, userIDAttr, "share", share, "error", err)
			s.Brokers.SendToast(models.NewErrorGeneralToast(msg), userID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		slog.Info("Created share link", userIDAttr, "share", share, "link", link)

		_ = components.ShareLink(templates.Data{
			Content: r.Host + link,
		}).Render(r.Context(), w)
	}
}

func (s *Server) recipesSearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()

		page, err := strconv.ParseUint(query.Get("page"), 10, 64)
		if err != nil || page <= 0 {
			page = 1
		}

		q := query.Get("q")
		sorts := query.Get("sort")

		if q == "" {
			w.Header().Set("HX-Redirect", "/recipes?sort="+sorts)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Query parameter must not be 'q' empty."))
			return
		}
		q = strings.ReplaceAll(q, ",", " ")
		q = strings.Join(strings.Fields(q), " ")

		userID := getUserID(r)

		mode := query.Get("mode")
		if mode == "" {
			s.Brokers.SendToast(models.NewErrorGeneralToast("Missing query parameter 'method'. Valid values are 'name' or 'full'."), userID)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		opts := models.NewSearchOptionsRecipe(mode, sorts, page)

		recipes, totalCount, err := s.Repository.SearchRecipes(q, page, opts, userID)
		if err != nil {
			s.Brokers.SendToast(models.NewErrorGeneralToast("Error searching recipes."), userID)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(recipes) == 0 {
			_ = components.SearchNoResult().Render(r.Context(), w)
			return
		}

		numPages := totalCount / templates.ResultsPerPage
		if numPages == 0 {
			numPages = 1
		}

		isHxReq := r.Header.Get("HX-Request") == "true"
		params := "q=" + q + "&mode=" + mode + "&sort=" + sorts
		htmx := templates.PaginationHtmx{IsSwap: isHxReq, Target: "#list-recipes"}

		p := templates.NewPagination(page, numPages, totalCount, templates.ResultsPerPage, "/recipes/search", params, htmx)
		p.Search.CurrentPage = page

		_ = components.RecipesSearch(templates.Data{
			About:           templates.NewAboutData(),
			IsAdmin:         userID == 1,
			IsAutologin:     app.Config.Server.IsAutologin,
			IsAuthenticated: true,
			IsHxRequest:     isHxReq,
			Functions:       templates.NewFunctionsData[int64](),
			Pagination:      p,
			Recipes:         recipes,
			Searchbar: templates.SearchbarData{
				Mode: mode,
				Sort: sorts,
				Term: q,
			},
		}).Render(r.Context(), w)
	}
}

func (s *Server) recipesSupportedApplicationsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		applications := [][]string{
			{"AccuChef", "https://www.accuchef.com"},
			{"ChefTap", "https://cheftap.com"},
			{"Crouton", "https://crouton.app"},
			{"Easy Recipe Deluxe", "https://easy-recipe-deluxe.software.informer.com"},
			{"Kalorio", "https://www.kalorio.de"},
			{"MasterCook", "https://www.mastercook.com"},
			{"Paprika", "https://www.paprikaapp.com"},
			{"Recipe Keeper", "https://recipekeeperonline.com"},
			{"RecipeSage", "https://recipesage.com"},
			{"Saffron", "https://www.mysaffronapp.com"},
		}

		var sb strings.Builder
		for i, application := range applications {
			tr := `<tr class="border text-center">`
			data1 := `<td class="border dark:border-gray-800">` + strconv.Itoa(i+1) + "</td>"
			link := `<a class="underline" href="` + application[1] + `" target="_blank">` + application[0] + "</a>"
			data2 := `<td class="border py-1 dark:border-gray-800">` + link + "</td>"
			sb.WriteString(tr + data1 + data2 + "</tr>")
		}

		if sb.Len() == 0 {
			tr := `<tr class="border px-8 py-2">`
			data1 := `<td class="border dark:border-gray-800">-1</td>`
			data2 := `<td class="border py-1 dark:border-gray-800">No result</td>`
			sb.WriteString(tr + data1 + data2 + "</tr>")
		}

		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, sb.String())
	}
}

func (s *Server) recipesSupportedWebsitesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		websites := s.Repository.Websites()
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, websites.TableHTML())
	}
}

func (s *Server) recipesViewHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePathPositiveID(r.PathValue("id"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		userID := getUserID(r)
		recipe, err := s.Repository.Recipe(id, userID)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		_ = components.ViewRecipe(templates.Data{
			About:           templates.NewAboutData(),
			IsAdmin:         userID == 1,
			IsAuthenticated: true,
			IsHxRequest:     r.Header.Get("Hx-Request") == "true",
			View:            templates.NewViewRecipeData(id, recipe, true, false),
		}).Render(r.Context(), w)
	}
}

func (s *Server) recipesViewShareHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parsePathPositiveID(r.PathValue("id"))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		cookbookIDStr := r.URL.Query().Get("cookbook")
		cookbookID, err := strconv.ParseInt(cookbookIDStr, 10, 64)
		if err != nil {
			s.Brokers.SendToast(models.NewErrorGeneralToast("Could not parse cookbookID query parameter."), getUserID(r))
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		recipe, cookbookUserID, err := s.Repository.CookbookRecipe(id, cookbookID)
		if err != nil {
			notFoundHandler(w, r)
			return
		}

		userID, isLoggedIn := s.findUserID(r)

		_ = components.ViewRecipe(templates.Data{
			About:           templates.NewAboutData(),
			IsAdmin:         userID == 1,
			IsAuthenticated: isLoggedIn,
			IsHxRequest:     r.Header.Get("Hx-Request") == "true",
			View:            templates.NewViewRecipeData(id, recipe, cookbookUserID == userID, true),
		}).Render(r.Context(), w)
	}
}
