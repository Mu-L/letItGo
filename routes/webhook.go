package routes

import (
	"net/http"

	"github.com/Sumit189letItGo/controllers"
	"github.com/gorilla/mux"
)

func WebhookRoutes(router *mux.Router) {
	router.HandleFunc("/schedule", WebhookHandler).Methods("POST")
}

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	controllers.ScheduleHandler(w, r)
}
