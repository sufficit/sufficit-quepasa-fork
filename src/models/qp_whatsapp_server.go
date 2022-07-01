package models

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/uuid"
	whatsapp "github.com/sufficit/sufficit-quepasa-fork/whatsapp"
)

type QPWhatsappServer struct {
	Bot            *QPBot                       `json:"bot"`
	connection     whatsapp.IWhatsappConnection `json:"-"`
	syncConnection *sync.Mutex                  `json:"-"` // Objeto de sinaleiro para evitar chamadas simultâneas a este objeto
	syncMessages   *sync.Mutex                  `json:"-"` // Objeto de sinaleiro para evitar chamadas simultâneas a este objeto
	Battery        WhatsAppBateryStatus         `json:"battery"`
	Timestamp      time.Time                    `json:"starttime"`
	Handler        *QPWhatsappHandlers          `json:"-"`

	stopRequested bool        `json:"-"`
	logger        *log.Logger `json:"-"`
	Log           *log.Entry  `json:"-"`
}

//region CONSTRUCTORS

// Instanciando um novo servidor para controle de whatsapp
func NewQPWhatsappServer(bot *QPBot) (server *QPWhatsappServer, err error) {
	wid := bot.ID
	var serverLogLevel log.Level
	if bot.Devel {
		serverLogLevel = log.DebugLevel
	} else {
		serverLogLevel = log.InfoLevel
	}

	serverLogger := log.New()
	serverLogger.SetLevel(serverLogLevel)
	serverLogEntry := serverLogger.WithField("wid", wid)

	handler := NewQPWhatsappHandlers(bot.HandleGroups, bot.HandleBroadcast, serverLogEntry)
	server = &QPWhatsappServer{
		Bot:            bot,
		syncConnection: &sync.Mutex{},
		syncMessages:   &sync.Mutex{},
		Battery:        WhatsAppBateryStatus{},
		Timestamp:      time.Now(),
		Handler:        handler,

		stopRequested: false,
		logger:        serverLogger,
		Log:           serverLogEntry,
	}
	return
}

//endregion
//region IMPLEMENT OF INTERFACE STATE RECOVERY

func (server *QPWhatsappServer) GetStatus() whatsapp.WhatsappConnectionState {
	if server.connection != nil {
		return server.connection.GetStatus()
	} else if server.stopRequested {
		return whatsapp.Stopped
	} else {
		return whatsapp.Created
	}
}

//endregion
//region IMPLEMENT OF INTERFACE QUEPASA SERVER

// Returns whatsapp controller id on E164
// Ex: 5521967609095
func (server *QPWhatsappServer) GetWid() string {
	return server.Bot.ID
}

func (server *QPWhatsappServer) DownloadData(id string) ([]byte, error) {
	msg, err := server.Handler.GetMessage(id)
	if err != nil {
		return nil, err
	}

	server.Log.Infof("downloading msg %s", id)
	return server.connection.DownloadData(&msg)
}

func (server *QPWhatsappServer) Download(id string) (att whatsapp.WhatsappAttachment, err error) {
	msg, err := server.Handler.GetMessage(id)
	if err != nil {
		return
	}

	server.Log.Infof("downloading msg %s", id)
	return server.connection.Download(&msg)
}

func (server *QPWhatsappServer) Send(recipient string, text string) (msg whatsapp.IWhatsappSendResponse, err error) {
	chat := whatsapp.WhatsappChat{ID: recipient}
	msg = &whatsapp.WhatsappMessage{
		Text: text,
		Chat: chat,
	}

	err = server.SendMessage(msg.(*whatsapp.WhatsappMessage))
	return
}

func (server *QPWhatsappServer) SendAttachment(recipient string, text string, attach whatsapp.WhatsappAttachment) (msg whatsapp.IWhatsappSendResponse, err error) {
	chat := whatsapp.WhatsappChat{ID: recipient}
	msg = &whatsapp.WhatsappMessage{
		Text:       text,
		Chat:       chat,
		Attachment: &attach,
	}

	err = server.SendMessage(msg.(*whatsapp.WhatsappMessage))
	return
}

func (server *QPWhatsappServer) SendMessage(msg *whatsapp.WhatsappMessage) (err error) {
	server.Log.Debugf("sending msg to: %v", msg.Chat.ID)
	response, err := server.connection.Send(*msg)
	msg.ID = response.GetID()
	msg.Timestamp = response.GetTime()
	return
}

//endregion

func (server *QPWhatsappServer) GetMessages(timestamp time.Time) (messages []whatsapp.WhatsappMessage, err error) {
	for _, item := range server.Handler.GetMessages(timestamp) {
		messages = append(messages, item)
	}
	return
}

// Roda de forma assíncrona, não interessa o resultado ao chamador
// Inicia o processo de tentativas de conexão de um servidor individual
func (server *QPWhatsappServer) Initialize() {
	if server == nil {
		panic("nil server, code error")
	}

	server.Log.Info("Initializing WhatsApp Server ...")
	err := server.Start()
	if err != nil {
		server.Log.Error(err)
	}
}

// Update underlying connection and ensure trivials
func (server *QPWhatsappServer) UpdateConnection(connection whatsapp.IWhatsappConnection) {
	if server.connection != nil {
		defer server.connection.Dispose()
	}

	server.connection = connection
	server.connection.UpdateLog(server.Log)
	if server.Handler == nil {
		server.logger.Info("creating handlers ?!")
	}

	server.connection.UpdateHandler(server.Handler)

	// Registrando webhook
	webhookDispatcher := &QPWebhookHandlerV2{Server: server}
	server.Handler.Register(webhookDispatcher)

	go server.EnsureHandlers()
}

func (server *QPWhatsappServer) EnsureHandlers() error {
	return server.connection.EnsureHandlers()
}

func (server *QPWhatsappServer) EnsureUnderlying() (err error) {
	server.syncConnection.Lock()

	// conectar dispositivo
	if server.connection == nil {
		server.Log.Infof("trying to create new whatsapp connection ...")
		wid := server.GetWid()
		log := server.logger

		var connection whatsapp.IWhatsappConnection
		connection, err = NewWhatsmeowConnection(wid, log)

		server.connection = connection
	}

	server.syncConnection.Unlock()
	return
}

func (server *QPWhatsappServer) Start() (err error) {
	err = server.EnsureUnderlying()
	if err != nil {
		return
	}

	server.Log.Infof("Starting WhatsApp Server ...")

	if server.GetWorking() {
		state := server.GetStatus()
		server.Log.Warnf("trying to start a server on an invalid state :: %s", state)
		return
	}

	// Registrando webhook
	webhookDispatcher := &QPWebhookHandlerV2{Server: server}
	server.Handler.Register(webhookDispatcher)

	// Atualizando manipuladores de eventos
	server.connection.UpdateHandler(server.Handler)

	server.Log.Infof("Requesting connection ...")
	err = server.connection.Connect()
	if err != nil {
		if unauthorized, ok := err.(*whatsapp.UnauthorizedError); ok {
			server.Log.Warningf("unauthorized, setting unverified")
			err = unauthorized

			err = server.Bot.UpdateVerified(false)
		}
		return
	}

	server.MarkVerified(true)
	return
}

func (server *QPWhatsappServer) Restart() {
	server.Log.Info("restart requested ....")
	// Somente executa caso não esteja em estado de processo de conexão
	// Evita chamadas simultâneas desnecessárias
	/*
		if server.Status != Starting {
			server.Status = Restarting
			server.log.Infof("Restarting WhatsApp Server ...")

			server.Connection.Disconnect()
			server.Status = Disconnected

			// Inicia novamente o servidor e os Handlers(alças)
			err := server.Initialize()
			if err != nil {
				server.Status = Failed
				server.log.Infof("(ERR) Critical error on WhatsApp Server: %s", err.Error())
			}
		}
	*/
}

// Somente usar em caso de não ser permitida a reconxão automática
func (server *QPWhatsappServer) Disconnect(cause string) {
	server.Log.Infof("Disconnecting WhatsApp Server: %s", cause)

	if server.connection != nil {
		server.connection.Disconnect()
	}
}

// Retorna o titulo em cache (se houver) do id passado em parametro
func (server *QPWhatsappServer) GetTitle(Wid string) string {
	return server.connection.GetTitle(Wid)
}

// Usado para exibir os servidores/bots de cada usuario em suas respectivas telas
func (server *QPWhatsappServer) GetOwnerID() string {
	return server.Bot.UserID
}

//region QP BOT EXTENSIONS

func (server *QPWhatsappServer) GetWorking() bool {
	status := server.GetStatus()
	if status <= whatsapp.Stopped {
		return false
	} else if status == whatsapp.Disconnected {
		return false
	}
	return true
}

func (server *QPWhatsappServer) GetStatusString() string {
	return server.GetStatus().String()
}

func (server *QPWhatsappServer) ID() string {
	return server.Bot.ID
}

// Traduz o Wid para um número de telefone em formato E164
func (server *QPWhatsappServer) GetNumber() string {
	return server.Bot.GetNumber()
}

func (server *QPWhatsappServer) GetTimestamp() (timestamp uint64) {
	return server.Bot.GetTimestamp()
}

func (server *QPWhatsappServer) GetStartedTime() (timestamp time.Time) {
	return server.Bot.GetStartedTime()
}

func (server *QPWhatsappServer) GetBatteryInfo() WhatsAppBateryStatus {
	return server.Bot.GetBatteryInfo()
}

func (server *QPWhatsappServer) GetConnection() whatsapp.IWhatsappConnection {
	return server.connection
}

func (server *QPWhatsappServer) Toggle() (err error) {
	if !server.GetWorking() {
		server.stopRequested = false
		err = server.Start()
	} else {
		server.stopRequested = true

		server.Disconnect("toggling")
	}
	return
}

func (server *QPWhatsappServer) IsDevelopmentGlobal() bool {
	return ENV.IsDevelopment()
}

func (server *QPWhatsappServer) CycleToken() (err error) {
	value := uuid.New().String()
	err = server.Bot.UpdateToken(value)
	if err != nil {
		return
	}

	server.Log.Infof("cycling token: %v", value)
	return
}

func (server *QPWhatsappServer) Token() string {
	return server.Bot.Token
}

func (server *QPWhatsappServer) MarkVerified(value bool) (err error) {
	err = server.Bot.UpdateVerified(value)
	if err != nil {
		return
	}

	server.Log.Infof("updating verified status: %v", value)
	return
}

func (server *QPWhatsappServer) Verified() bool {
	return server.Bot.Verified
}

func (server *QPWhatsappServer) ToggleGroups() (err error) {
	err = server.Bot.UpdateGroups(!server.Bot.HandleGroups)
	if err != nil {
		return
	}

	server.Handler.HandleGroups = server.Bot.HandleGroups
	server.Log.Infof("toggling handler of group messages: %v", server.Handler.HandleGroups)
	return
}

func (server *QPWhatsappServer) HandleGroups() bool {
	return server.Bot.HandleGroups
}

func (server *QPWhatsappServer) ToggleBroadcast() (err error) {
	err = server.Bot.UpdateBroadcast(!server.Bot.HandleBroadcast)
	if err != nil {
		return
	}

	server.Handler.HandleBroadcast = server.Bot.HandleBroadcast

	server.Log.Infof("toggling handler of broadcast messages: %v", server.Handler.HandleBroadcast)
	return
}

func (server *QPWhatsappServer) HandleBroadcast() bool {
	return server.Bot.HandleBroadcast
}

func (server *QPWhatsappServer) ToggleDevel() (err error) {
	err = server.Bot.UpdateDevel(!server.Bot.Devel)
	if err != nil {
		return
	}

	if server.Bot.Devel {
		server.logger.SetLevel(log.DebugLevel)
	} else {
		server.logger.SetLevel(log.InfoLevel)
	}

	server.Log.Infof("toggling logger level: %v", server.logger.Level)
	return
}

func (server *QPWhatsappServer) Devel() bool {
	return server.Bot.Devel
}

func (server *QPWhatsappServer) SetWebhook(value string) error {
	return server.Bot.UpdateWebhook(value)
}

func (server *QPWhatsappServer) Webhook() string {
	return server.Bot.Webhook
}

func (server *QPWhatsappServer) SetVersion(value string) error {
	return server.Bot.UpdateVersion(value)
}

func (server *QPWhatsappServer) Version() string {
	return server.Bot.Version
}

//endregion

func (server *QPWhatsappServer) Delete() (err error) {
	if server.connection != nil {
		err = server.connection.Delete()
		if err != nil {
			return
		}
	}

	return server.Bot.Delete()
}

func (server *QPWhatsappServer) WebHookSincronize() error {
	return server.Bot.WebHookSincronize()
}

//endregion