package notifier

import (
	"bytes"
)

type Notifier interface {
	NotifyServer(serverName string, message string)
	NotifyAndAttachServerData(serverName string, message string, attachmentName string, attachment *bytes.Buffer)
}
