package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
	. "github.com/sufficit/sufficit-quepasa-fork/metrics"
	. "github.com/sufficit/sufficit-quepasa-fork/models"
	. "github.com/sufficit/sufficit-quepasa-fork/whatsapp"
)

// Renders route GET "/{version}/bot/{token}/receive"
func ReceiveAPIHandlerV2(w http.ResponseWriter, r *http.Request) {

	// setting default reponse type as json
	w.Header().Set("Content-Type", "application/json")

	token := chi.URLParam(r, "token")
	server, err := GetServerFromToken(token)
	if err != nil {
		RespondNotFound(w, fmt.Errorf("Token '%s' not found", token))
		return
	}

	// Evitando tentativa de download de anexos sem o bot estar devidamente sincronizado
	status := server.GetStatus()
	if status != Ready {
		RespondNotReady(w, &ApiServerNotReadyException{Wid: server.GetWid(), Status: status})
		return
	}

	queryValues := r.URL.Query()
	timestamp := queryValues.Get("timestamp")

	messages, err := GetMessagesToAPIV2(server, timestamp)
	if err != nil {
		MessageReceiveErrors.Inc()
		RespondServerError(server, w, err)
		return
	}

	MessagesReceived.Add(float64(len(messages)))

	out := QPFormReceiveResponseV2{
		Bot:      *ToQPBotV2(server.Bot),
		Messages: messages,
	}

	RespondSuccess(w, out)
}

// SendAPIHandler renders route "/v2/bot/{token}/send"
func SendTextAPIHandlerV2(w http.ResponseWriter, r *http.Request) {

	// setting default reponse type as json
	w.Header().Set("Content-Type", "application/json")

	token := chi.URLParam(r, "token")
	server, err := GetServerFromToken(token)
	if err != nil {
		RespondNotFound(w, fmt.Errorf("Token '%s' not found", token))
		return
	}

	// Declare a new Person struct.
	var request QPSendRequest

	// Try to decode the request body into the struct. If there is an error,
	// respond to the client with the error message and a 400 status code.
	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		RespondServerError(server, w, err)
		return
	}

	log.Tracef("sending requested: %v", request)
	recipient, err := FormatEndpoint(request.Recipient)
	if err != nil {
		MessageSendErrors.Inc()
		return
	}

	sendResponse, err := server.Send(recipient, request.Message)
	if err != nil {
		MessageSendErrors.Inc()
		RespondServerError(server, w, err)
		return
	}

	response := QPSendResponseV2{}
	response.Chat.ID = request.Recipient
	response.Chat.UserName = recipient
	response.Chat.Title = server.GetTitle(recipient)
	response.From.ID = server.Bot.ID
	response.From.UserName = server.Bot.GetNumber()
	response.ID = sendResponse.GetID()

	// Para manter a compatibilidade
	response.PreviusV1 = QPSendResult{
		Source:    server.GetWid(),
		Recipient: recipient,
		MessageId: sendResponse.GetID(),
	}

	MessagesSent.Inc()
	RespondSuccess(w, response)
}

// Usado para envio de documentos, anexos, separados do texto, em caso de imagem, aceita um caption (titulo)
func SendDocumentAPIHandlerV2(w http.ResponseWriter, r *http.Request) {

	// setting default reponse type as json
	w.Header().Set("Content-Type", "application/json")

	token := chi.URLParam(r, "token")
	server, err := GetServerFromToken(token)
	if err != nil {
		MessageSendErrors.Inc()
		RespondNotFound(w, fmt.Errorf("Token '%s' not found", token))
		return
	}

	// Declare a new Person struct.
	var request QPSendDocumentRequestV2

	// Try to decode the request body into the struct. If there is an error,
	// respond to the client with the error message and a 400 status code.
	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		MessageSendErrors.Inc()
		RespondServerError(server, w, err)
		return
	}

	if request.Attachment == (QPAttachmentV1{}) {
		MessageSendErrors.Inc()
		RespondServerError(server, w, fmt.Errorf("attachment not found"))
		return
	}

	recipient, err := FormatEndpoint(request.Recipient)
	if err != nil {
		MessageSendErrors.Inc()
		return
	}

	attach, err := ToWhatsappAttachment(&request.Attachment)
	sendResponse, err := server.SendAttachment(recipient, request.Message, *attach)
	if err != nil {
		MessageSendErrors.Inc()
		RespondServerError(server, w, err)
		return
	}

	response := QPSendResponseV2{}
	response.Chat.ID = request.Recipient
	response.Chat.UserName = recipient
	response.Chat.Title = server.GetTitle(recipient)
	response.From.ID = server.Bot.ID
	response.From.UserName = server.Bot.GetNumber()
	response.ID = sendResponse.GetID()

	// Para manter a compatibilidade
	response.PreviusV1 = QPSendResult{
		Source:    server.GetWid(),
		Recipient: recipient,
		MessageId: sendResponse.GetID(),
	}

	MessagesSent.Inc()
	RespondSuccess(w, response)
}

// AttachmentHandler renders route POST "/v1/bot/{token}/attachment"
func AttachmentAPIHandlerV2(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	server, err := GetServerFromToken(token)
	if err != nil {
		RespondNotFound(w, fmt.Errorf("Token '%s' not found", token))
		return
	}

	// Evitando tentativa de download de anexos sem o bot estar devidamente sincronizado
	status := server.GetStatus()
	if status != Ready {
		RespondNotReady(w, &ApiServerNotReadyException{Wid: server.GetWid(), Status: status})
		return
	}

	// Declare a new Person struct.
	var p QPAttachmentV1

	// Try to decode the request body into the struct. If there is an error,
	// respond to the client with the error message and a 400 status code.
	err = json.NewDecoder(r.Body).Decode(&p)
	if err != nil {
		RespondServerError(server, w, err)
	}

	ss := strings.Split(p.Url, "/")
	id := ss[len(ss)-1]

	att, err := server.Download(id)
	if err != nil {
		RespondServerError(server, w, err)
		return
	}

	if len(att.FileName) > 0 {
		w.Header().Set("Content-Disposition", "attachment; filename="+att.FileName)
	}

	if len(att.Mimetype) > 0 {
		w.Header().Set("Content-Type", att.Mimetype)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(*att.GetContent())
}
