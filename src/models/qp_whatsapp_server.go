package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/google/uuid"
	. "github.com/sufficit/sufficit-quepasa-fork/whatsapp"
)

type QPWhatsappServer struct {
	Bot            *QPBot               `json:"bot"`
	Connection     IWhatsappConnection  `json:"-"`
	syncConnection *sync.Mutex          `json:"-"` // Objeto de sinaleiro para evitar chamadas simultâneas a este objeto
	syncMessages   *sync.Mutex          `json:"-"` // Objeto de sinaleiro para evitar chamadas simultâneas a este objeto
	Status         QPWhatsappState      `json:"status"`
	Battery        WhatsAppBateryStatus `json:"battery"`
	Timestamp      time.Time            `json:"starttime"`
	Handler        *QPWhatsappHandlers  `json:"-"`
	logger         *log.Logger          `json:"-"`
	log            *log.Entry           `json:"-"`
}

//region CONSTRUCTORS

// Instanciando um novo servidor para controle de whatsapp
func NewQPWhatsappServer(bot *QPBot, connection IWhatsappConnection) (server *QPWhatsappServer, err error) {
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

	if connection == nil {
		// Definindo conexão com whatsapp
		connection, err = NewConnection(wid, serverLogger)
		if err != nil {
			return
		}
	}

	state := Created
	handler := NewQPWhatsappHandlers(bot.HandleGroups, bot.HandleBroadcast)

	server = &QPWhatsappServer{
		Bot:            bot,
		Connection:     connection,
		syncConnection: &sync.Mutex{},
		syncMessages:   &sync.Mutex{},
		Status:         state,
		Battery:        WhatsAppBateryStatus{},
		Timestamp:      time.Now(),
		Handler:        handler,
		logger:         serverLogger,
		log:            serverLogEntry,
	}

	return
}

//endregion
//region IMPLEMENT OF INTERFACE STATE RECOVERY

func (server *QPWhatsappServer) GetStatus() QPWhatsappState {
	return server.Status
}

//endregion
//region IMPLEMENT OF INTERFACE QUEPASA SERVER

// Returns whatsapp controller id on E164
// Ex: 5521967609095
func (server *QPWhatsappServer) GetWid() string {
	return server.Bot.ID
}

func (server *QPWhatsappServer) Download(id string) ([]byte, error) {
	msg, err := server.Handler.GetMessage(id)
	if err != nil {
		return nil, err
	}

	return server.Connection.Download(&msg)
}

func (server *QPWhatsappServer) Send(recipient string, text string) (msg IWhatsappSendResponse, err error) {
	chat := WhatsappChat{ID: recipient}
	msg = &WhatsappMessage{
		Text: text,
		Chat: chat,
	}

	err = server.SendMessage(msg.(*WhatsappMessage))
	return
}

func (server *QPWhatsappServer) SendAttachment(recipient string, text string, attach WhatsappAttachment) (msg IWhatsappSendResponse, err error) {
	chat := WhatsappChat{ID: recipient}
	msg = &WhatsappMessage{
		Text:       text,
		Chat:       chat,
		Attachment: &attach,
	}

	err = server.SendMessage(msg.(*WhatsappMessage))
	return
}

func (server *QPWhatsappServer) SendMessage(msg *WhatsappMessage) (err error) {
	response, err := server.Connection.Send(*msg)
	msg.ID = response.GetID()
	msg.Timestamp = response.GetTime()
	return
}

//endregion

func (server *QPWhatsappServer) GetMessages(timestamp time.Time) (messages []WhatsappMessage, err error) {
	for _, item := range server.Handler.GetMessages(timestamp) {
		messages = append(messages, item)
	}
	return
}

// Inicializa um repetidor eterno que confere o estado da conexão e tenta novamente a cada 10 segundos
func (server *QPWhatsappServer) Initialize() (err error) {
	server.log.Info("Initializing WhatsApp Server ...")
	for {
		err = server.Start()
		if err == nil {
			break
		} else {
			server.log.Warn("Error on initializing")
		}

		// Aguardaremos 10 segundos e vamos tentar novamente
		time.Sleep(10 * time.Second)
	}
	return nil
}

// Inicializa um repetidor eterno que confere o estado da conexão e tenta novamente a cada 10 segundos
func (server *QPWhatsappServer) Shutdown() (err error) {
	//server.syncConnection.Lock() // Travando

	server.Status = Halting
	server.log.Info("Shutting Down WhatsApp Server ...")

	err = server.Connection.Disconnect()

	// caso erro diferente de nulo e não seja pq já esta desconectado
	if err != nil && !strings.Contains(err.Error(), "not connected") {
		server.log.Error("Shutting WhatsApp Server", err)
	} else {
		server.Status = Stopped
	}

	//server.syncConnection.Unlock() // Destravando
	return
}

func (server *QPWhatsappServer) Start() (err error) {
	server.syncConnection.Lock() // Travando

	state := server.GetStatus()
	if state != Created && state != Stopped {
		err = fmt.Errorf("(%s) trying to start a server not an created|stopped state", server.Bot.ID)
		return
	}

	server.Status = Starting
	server.log.Infof("Starting WhatsApp Server ...")

	// conectar dispositivo
	if server.Connection == nil {
		err = fmt.Errorf("(%s) null connection on trying to start server", server.Bot.ID)
	} else {

		// Registrando webhook
		webhookDispatcher := &QPWebhookHandlerV2{Server: server}
		server.Handler.Register(webhookDispatcher)

		// Atualizando manipuladores de eventos
		server.Connection.UpdateHandler(server.Handler)
		if err != nil {
			err = server.MarkVerified(false)
			return
		}

		server.log.Infof("Requesting connection ...")
		err = server.Connection.Connect()
		if err != nil {
			return
		}

		// Inicializando conexões e handlers
		err = server.startHandlers()
		if err != nil {
			server.Status = Failed
			switch err.(type) {
			default:
				if strings.Contains(err.Error(), "401") {
					server.log.Infof(" WhatsApp return a unauthorized state, please verify again")
					err = server.MarkVerified(false)
				} else if strings.Contains(err.Error(), "restore session connection timed out") {
					server.log.Infof("WhatsApp returns after a timeout, trying again in 10 seconds, please wait ...")
				} else {
					server.log.Infof("(ERR) SUFF ERROR F :: Starting Handlers error ... %s :", err)
				}
			case *ServiceUnreachableError:
				server.log.Error(err)
			}

			// Importante para evitar que a conexão em falha continue aberta
			server.Connection.Disconnect()

		} else {
			server.Status = Ready
		}
	}

	server.syncConnection.Unlock() // Destravando
	return
}

func (server *QPWhatsappServer) Restart() {
	// Somente executa caso não esteja em estado de processo de conexão
	// Evita chamadas simultâneas desnecessárias
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
}

// Somente usar em caso de não ser permitida a reconxão automática
func (server *QPWhatsappServer) Disconnect(cause string) {
	server.log.Infof("Disconnecting WhatsApp Server: %s", cause)

	server.syncConnection.Lock() // Travando
	// ------

	server.Connection.Disconnect()

	// ------
	server.syncConnection.Unlock() // Destravando
}

func (server *QPWhatsappServer) startHandlers() (err error) {
	// Definindo handlers para mensagens assincronas
	//startupHandler := &QPMessageHandler{&server.Bot, true, server}
	//con.AddHandler(startupHandler)

	// Atualizando informação sobre o estado da conexão e do servidor
	server.Status = Connected

	// Aguarda 3 segundos
	<-time.After(3 * time.Second)

	server.Status = Fetching
	server.log.Infof("Setting up long-running message handler")
	//asyncMessageHandler := &QPMessageHandler{&server.Bot, true, server}
	//server.Handlers = *asyncMessageHandler
	//con.AddHandler(asyncMessageHandler)
	return
}

// Retorna o titulo em cache (se houver) do id passado em parametro
func (server *QPWhatsappServer) GetTitle(Wid string) string {
	return server.Connection.GetTitle()
}

// Usado para exibir os servidores/bots de cada usuario em suas respectivas telas
func (server *QPWhatsappServer) GetOwnerID() string {
	return server.Bot.UserID
}

//region QP BOT EXTENSIONS

func (server *QPWhatsappServer) GetStatusString() string {
	return server.Bot.GetStatus()
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

func (server *QPWhatsappServer) GetBatteryInfo() (status WhatsAppBateryStatus) {
	return server.Bot.GetBatteryInfo()
}

func (server *QPWhatsappServer) Toggle() (err error) {
	return server.Bot.Toggle()
}

func (server *QPWhatsappServer) IsDevelopmentGlobal() bool {
	return ENV.IsDevelopment()
}

//region SINGLE UPDATES

/*
UpdateToken(id string, value string) error
UpdateGroups(id string, value bool) error
UpdateBroadcast(id string, value bool) error
UpdateVerified(id string, value bool) error
UpdateWebhook(id string, value string) error
UpdateDevel(id string, value bool) error
UpdateVersion(id string, value string) error
*/

func (server *QPWhatsappServer) CycleToken() (err error) {
	value := uuid.New().String()
	err = server.Bot.UpdateToken(value)
	if err != nil {
		return
	}

	server.log.Infof("cycling token: %v", value)
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

	server.log.Infof("updating verified status: %v", value)
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
	server.log.Infof("toggling handler of group messages: %v", server.Handler.HandleGroups)
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

	server.log.Infof("toggling handler of broadcast messages: %v", server.Handler.HandleBroadcast)
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

	server.log.Infof("toggling logger level: %v", server.logger.Level)
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
	err = server.Bot.Delete()
	if err != nil {
		return
	}

	server.Disconnect("server deleted")
	err = server.Shutdown()
	if err != nil {
		return
	}

	return server.Connection.Delete()
}

func (server *QPWhatsappServer) WebHookSincronize() error {
	return server.Bot.WebHookSincronize()
}

//endregion
