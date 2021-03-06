package models

type QpDataWebhookInterface interface {
	Find(context string, url string) (*QpBotWebhook, error)
	FindAll(context string) ([]*QpBotWebhook, error)
	All() ([]QpBotWebhook, error)
	Add(context string, url string, forwardinternal bool, trackid string) error
	Update(element QpBotWebhook) error
	Remove(context string, url string) error
	Clear(context string) error
}
