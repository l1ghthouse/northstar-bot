package notifier

import (
	"bytes"

	"github.com/l1ghthouse/northstar-bootstrap/src/nsserver"
)

type Notifier interface {
	NotifyServer(serverName *nsserver.NSServer, message string)
	NotifyAndAttachServerData(serverName *nsserver.NSServer, message string, attachmentName string, attachment *bytes.Buffer)
}
