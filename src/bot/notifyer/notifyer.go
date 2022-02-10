package notifyer

import "io"

type Notifyer interface {
	Notify(message string)
	NotifyAndAttach(message string, attachmentName string, attachment io.Reader)
}
