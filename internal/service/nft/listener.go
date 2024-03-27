package nft

import (
	"errors"
	"strconv"
	"time"
)

const LISTEN_TIME_INTERVAL_SECOND = 1 * 60

type Listener interface {
	AddListenItem(data any)
}

// ListenedData is the object that be listened
type ListenedData interface {
	GetHash() string
	Handle(item *ListenItem) *ListenItem
}

// Listen group for file status updata
type StatusListener struct {
	B        int
	QueueLen int
	Queues   []chan ListenItem
}

type ListenItem struct {
	Data  any
	Count int
	Timer *time.Timer
}

var listener *StatusListener

func GetStatusListener() Listener {
	return listener
}

func initStatusListener(b int, qLen int, listener *StatusListener) error {
	if b < 0 || qLen < 0 {
		return errors.New("invalid args")
	}
	listener.B = b
	listener.QueueLen = qLen
	listener.Queues = make([]chan ListenItem, 1<<b)
	for i := 0; i < len(listener.Queues); i++ {
		listener.Queues[i] = make(chan ListenItem, qLen)
		go listener.listenAndUpdate(i)
	}
	return nil
}

func (t *StatusListener) AddListenItem(data any) {
	ld, ok := data.(ListenedData)
	if !ok {
		logger.Error(errors.New("[File status update] can not prase data to ListenedData interface"), "")
		return
	}
	selector, err := strconv.ParseInt(ld.GetHash(), 16, 64)
	if err != nil {
		logger.Error(err, "[File status update] data hash", "fileHash", ld.GetHash())
		return
	}
	selector = selector % (1 << t.B)
	item := ListenItem{
		Data:  ld,
		Count: 0,
		Timer: time.NewTimer(time.Millisecond),
	}
	t.Queues[selector] <- item
	logger.Info("[File status update] data hash entry async listening", "fileHash", ld.GetHash())
}

func (t *StatusListener) listenAndUpdate(selecter int) {
	logger.Info("routine listener starting ...", "routineListener", selecter)
	for {
		queueLen := len(t.Queues[selecter])
		for i := 0; i < queueLen; i++ {
			item := <-t.Queues[selecter]
			select {
			case <-item.Timer.C:
				go func() {
					itemPtr := item.Data.(ListenedData).Handle(&item)
					if itemPtr != nil {
						t.Queues[selecter] <- *itemPtr
					}
				}()
			default:
				t.Queues[selecter] <- item
			}
		}
		time.Sleep(10 * time.Second)
	}
}
