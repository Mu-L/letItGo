package routes

import (
	"net/http"

	"github.com/Sumit189/letItGo/api/controllers"
	"github.com/gorilla/mux"
)

func ApiRoutes(router *mux.Router) {
	router.HandleFunc("/schedule", SchduleHandler).Methods("POST")
	router.HandleFunc("/webhook/verify", VerifyWebhookHandler).Methods("POST")
	router.NotFoundHandler = http.HandlerFunc(NotFoundHandler)
}

func SchduleHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	controllers.ScheduleHandler(ctx, w, r)
}

func VerifyWebhookHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	controllers.VerifyWebhookHandler(ctx, w, r)
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/404.html")
}
