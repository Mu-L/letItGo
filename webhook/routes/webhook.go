package routes

import (
	"net/http"

	"github.com/Sumit189/letItGo/webhook/controllers"
	"github.com/gorilla/mux"
)

func WebhookRoutes(router *mux.Router) {
	router.HandleFunc("/schedule", WebhookHandler).Methods("POST")
}

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	controllers.ScheduleHandler(ctx, w, r)
}
