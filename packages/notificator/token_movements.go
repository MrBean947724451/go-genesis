package notificator

import (
	"fmt"
	"strings"
	"time"

	"net/smtp"

	"github.com/GenesisKernel/go-genesis/packages/conf"
	"github.com/GenesisKernel/go-genesis/packages/consts"
	"github.com/GenesisKernel/go-genesis/packages/model"
	log "github.com/sirupsen/logrus"
)

const (
	networkPerDayLimit            = 100000000
	networkPerDayMsgTemplate      = "day APL movement volume =  %f"
	fromToDayLimitMsgTemplate     = "from %d to %d sended volume = %f"
	perBlockTokenMovementTemplate = "from wallet %d token movement count = %f in block: %d"

	networkPerDayEvent         = 1
	fromToDayLimitEvent        = 2
	perBlockTokenMovementEvent = 3
)

var lastLimitEvents map[uint8]time.Time

func init() {
	lastLimitEvents = make(map[uint8]time.Time, 0)
}

func sendEmail(conf conf.TokenMovementConfig, message string) error {
	auth := smtp.PlainAuth("", conf.Username, conf.Password, conf.Host)
	to := []string{conf.To}
	msg := []byte(fmt.Sprintf("From: %s\r\n", conf.From) +
		fmt.Sprintf("To: %s\r\n", conf.To) +
		fmt.Sprintf("Subject: %s\r\n", conf.Subject) +
		"\r\n" +
		fmt.Sprintf("%s\r\n", message))
	err := smtp.SendMail(fmt.Sprintf("%s:%d", conf.Host, conf.Port), auth, conf.From, to, msg)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("sending email")
	}
	return err
}

// CheckTokenMovementLimits check all limits
func CheckTokenMovementLimits(tx *model.DbTransaction, conf conf.TokenMovementConfig, blockID int64) {
	var messages []string
	if needCheck(networkPerDayEvent) {
		amount, err := model.GetExcessCommonTokenMovementPerDay(tx)

		if err != nil {
			log.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("check common token movement")
		} else if amount > networkPerDayLimit {
			messages = append(messages, fmt.Sprintf(networkPerDayMsgTemplate, amount))
			lastLimitEvents[networkPerDayEvent] = time.Now()
		}
	}

	if needCheck(fromToDayLimitEvent) {
		transfers, err := model.GetExcessFromToTokenMovementPerDay(tx)
		if err != nil {
			log.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("check from to token movement")
		} else {
			for _, transfer := range transfers {
				messages = append(messages, fmt.Sprintf(fromToDayLimitMsgTemplate, transfer.SenderID, transfer.RecipientID, transfer.Amount))
			}

			if len(transfers) > 0 {
				lastLimitEvents[fromToDayLimitEvent] = time.Now()
			}
		}
	}

	transfers, err := model.GetExcessTokenMovementQtyPerBlock(tx, blockID)
	if err != nil {
		log.WithFields(log.Fields{"type": consts.DBError, "error": err}).Error("check token movement per block")
	} else {
		for _, transfer := range transfers {
			messages = append(messages, fmt.Sprintf(perBlockTokenMovementTemplate, transfer.SenderID, transfer.Amount, blockID))
		}
	}

	if len(messages) > 0 {
		sendEmail(conf, strings.Join(messages, "\n"))
	}
}

// checks needed only if we have'nt prevent events or if event older then 1 day
func needCheck(event uint8) bool {
	t, ok := lastLimitEvents[event]
	if !ok {
		return true
	}

	return time.Now().Sub(t) >= 24*time.Hour
}