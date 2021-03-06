module github.com/sufficit/sufficit-quepasa-fork/controllers

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/go-chi/chi v4.0.2+incompatible
	github.com/go-chi/jwtauth v4.0.4+incompatible
	github.com/gorilla/websocket v1.4.2
	github.com/nbutton23/zxcvbn-go v0.0.0-20210217022336-fa2cb2858354
	github.com/sirupsen/logrus v1.8.1
	github.com/sufficit/sufficit-quepasa-fork/library v0.0.0-00010101000000-000000000000
	github.com/sufficit/sufficit-quepasa-fork/metrics v0.0.0-00010101000000-000000000000
	github.com/sufficit/sufficit-quepasa-fork/models v0.0.0-00010101000000-000000000000
	github.com/sufficit/sufficit-quepasa-fork/whatsapp v0.0.0-00010101000000-000000000000
)

require (
	filippo.io/edwards25519 v1.0.0-rc.1 // indirect
	github.com/Rhymen/go-whatsapp v0.1.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/jmoiron/sqlx v1.2.0 // indirect
	github.com/joncalhoun/migrate v0.0.2 // indirect
	github.com/lib/pq v1.5.2 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.12.1 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.32.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/skip2/go-qrcode v0.0.0-20191027152451-9434209cb086 // indirect
	github.com/sufficit/sufficit-quepasa-fork/whatsmeow v0.0.0-00010101000000-000000000000 // indirect
	go.mau.fi/libsignal v0.0.0-20211109153248-a67163214910 // indirect
	go.mau.fi/whatsmeow v0.0.0-20220215120744-a1550ccceb70 // indirect
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292 // indirect
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
)

replace github.com/sufficit/sufficit-quepasa-fork/library => ../library

replace github.com/sufficit/sufficit-quepasa-fork/metrics => ../metrics

replace github.com/sufficit/sufficit-quepasa-fork/models => ../models

replace github.com/sufficit/sufficit-quepasa-fork/whatsapp => ../whatsapp

replace github.com/sufficit/sufficit-quepasa-fork/whatsmeow => ../whatsmeow

go 1.17
