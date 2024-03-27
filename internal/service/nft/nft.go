package nft

import (
	"strconv"
	"strings"
	"time"

	"vdo-platform/internal/app/ctx"
	"vdo-platform/internal/dto"
	"vdo-platform/internal/model"
	"vdo-platform/pkg/log"

	"github.com/go-logr/logr"
	"github.com/panjf2000/ants/v2"
	"github.com/pkg/errors"
)

var logger logr.Logger

func Setup() {
	logger = log.Logger
	fileStateQueryUrl = ctx.Settings.AppSetting.CmpHttpUrl
	listener = new(StatusListener)
	initStatusListener(3, 512, listener)
}

type TxData struct {
	IsSent bool
	Data   string
}

// creator, filename, filehash, description, cover, length string, size int64
func CreateVideoMetadata(req dto.CreateReq) (dto.CreateResp, error) {
	if req.Creator == "" || req.FileName == "" || req.CoverImage == "" || req.FileSize <= 0 {
		return dto.CreateResp{}, errors.New("there are empty video file parameters")
	}
	//check metadata is exsited
	v := &model.VideoMetadata{FileHash: req.FileHash}
	if yes, _ := v.IsExist(ctx.GormDb); yes {
		return dto.CreateResp{}, errors.New("video metadata is exist")
	}
	//create video metadata
	videoInfo := &model.VideoMetadata{
		Creator:     req.Creator,
		FileName:    req.FileName,
		FileHash:    req.FileHash,
		Description: req.Description,
		CoverImg:    req.CoverImage,
		Length:      req.Length,
		Size:        req.FileSize,
		Owner:       req.Creator,
		Label:       req.Label,
		Price:       model.NULL,
		NftStatus:   model.CREATE.String(),
		FileStatus:  model.UPLOAD.String(),
		Chain:       model.DEFAULT_CHAIN,
	}
	err := videoInfo.Create(ctx.GormDb)
	if err != nil {
		return dto.CreateResp{}, err
	}
	//create video file event
	videoEvent := model.Activity{
		EventType: model.ACT_FPG.String(),
		Creator:   req.Creator,
		FileHash:  req.FileHash,
		Source:    model.NULL,
		Target:    model.UPLOAD.String(),
		State:     model.SUCCESS.String(),
		StartDate: time.Now().Local().Format(ctx.Time_FMT),
		EndDate:   time.Now().Local().Format(ctx.Time_FMT),
	}
	err = videoEvent.Create(ctx.GormDb)
	if err != nil {
		return dto.CreateResp{}, err
	}
	//listen and update file status
	ants.Submit(func() {
		GetStatusListener().AddListenItem(NewMockFileStateHandler(req.FileHash))
	})
	//create event
	nftEvent := &model.Activity{
		EventType: model.ACT_CREATE.String(),
		Creator:   req.Creator,
		Source:    model.NULL,
		Target:    req.Creator,
		FileHash:  req.FileHash,
		State:     model.SUCCESS.String(),
		Price:     model.NULL,
		StartDate: videoEvent.StartDate,
		EndDate:   videoEvent.EndDate,
	}
	err = nftEvent.Create(ctx.GormDb)
	if err != nil {
		return dto.CreateResp{}, err
	}
	res := dto.CreateResp{
		Creator: req.Creator,
		Date:    videoEvent.EndDate,
	}
	return res, nil
}

func DeleteVideoMetadata(filehash string) error {
	//check metadata is exsited
	v := &model.VideoMetadata{FileHash: filehash}
	if yes, _ := v.IsExist(ctx.GormDb); !yes {
		return errors.New("video metadata is not exist")
	}
	if res, err := v.Get(ctx.GormDb); err == nil && len(res) > 0 {
		v = &res[0]
	}
	if v.NftStatus != model.CREATE.String() {
		return errors.New("video nft has been minted")
	}
	return v.Delete(ctx.GormDb)
}

func UpdateForMint(filehash string, data TxData) (dto.EventResp, error) {
	//check:once only
	var res dto.EventResp
	test := &model.VideoMetadata{FileHash: filehash, NftStatus: model.MINT.String()}
	if yes, err := test.IsExist(ctx.GormDb); yes || err != nil {
		return res, errors.New("nft already minted")
	}
	//query metadata
	nft := &model.VideoMetadata{FileHash: filehash}
	resp, err := nft.Get(ctx.GormDb)
	if err != nil || len(resp) != 1 {
		return res, errors.New("query nft metadata error")
	}
	nft = &resp[0]
	// if nft.FileStatus != model.STORAGE.String() {
	// 	return res, errors.New("video source file is pending,please wait a moment and try again")
	// }
	//create activity
	nftEvent := &model.Activity{
		EventType: model.ACT_MINT.String(),
		Creator:   nft.Creator,
		Source:    model.NULL,
		Target:    nft.Creator,
		FileHash:  filehash,
		State:     model.LISTENING.String(),
		NftToken:  filehash,
		Price:     model.NULL,
		StartDate: time.Now().Local().Format(ctx.Time_FMT),
	}
	err = nftEvent.Create(ctx.GormDb)
	if err != nil {
		return res, errors.Wrap(err, "create mint activity error")
	}
	//listen events
	// ants.Submit(func(){ zz(filehash, data, nftEvent, nft) })
	zz(filehash, data, nftEvent, nft)

	nftEvent.EndDate = time.Now().Local().Format(ctx.Time_FMT)
	res.EventType = nftEvent.EventType
	res.State = nftEvent.State
	res.Date = nftEvent.EndDate
	return res, nil
}

func zz(filehash string, data TxData, nftEvent *model.Activity, nft *model.VideoMetadata) {
	logger.Info("[Nft mint service] entry async listening...")
	var (
		txhash string
		err    error
	)
	if data.IsSent {
		txhash = data.Data
	} else {
		txhash, err = ctx.ChainClient.SendTx1(data.Data)
		if err != nil {
			nftEvent.State = model.FAILED.String()
			nftEvent.Update(ctx.GormDb)
			return
		}
	}
	//tx success,update activity state
	nftEvent.TxHash = txhash
	nftEvent.State = model.SUCCESS.String()
	if nftEvent.Update(ctx.GormDb) != nil {
		return
	}
	//change nft state
	nft.NftStatus = model.MINT.String()
	nft.NftToken = filehash
	//unexpected error,rollback
	if nft.Update(ctx.GormDb) != nil {
		nftEvent.State = model.FAILED.String()
		nftEvent.Update(ctx.GormDb)
	}
	logger.Info("[Nft mint service] exit async listening")
}

func UpdateForPurchase(filehash, to string, data TxData) (dto.EventResp, error) {
	//query metadata
	var res dto.EventResp
	nft := &model.VideoMetadata{FileHash: filehash}
	resp, err := nft.Get(ctx.GormDb)
	if err != nil || len(resp) != 1 {
		return res, errors.New("query nft metadata error")
	}
	nft = &resp[0]
	if nft.NftStatus != model.LIST.String() || nft.Price == model.NULL {
		return res, errors.New("nft not list or price error")
	}
	if to == nft.Owner {
		return res, errors.New("unable to purchase your own nft")
	}
	//create activity
	nftEvent := &model.Activity{
		EventType: model.ACT_TX.String(),
		Creator:   to,
		Source:    nft.Owner,
		Target:    to,
		FileHash:  filehash,
		State:     model.LISTENING.String(),
		NftToken:  nft.NftToken,
		Price:     nft.Price,
		StartDate: time.Now().Local().Format(ctx.Time_FMT),
	}
	err = nftEvent.Create(ctx.GormDb)
	if err != nil {
		return res, errors.Wrap(err, "create purchase activity error")
	}
	nftEvent.EndDate = time.Now().Local().Format(ctx.Time_FMT)
	res.EventType = nftEvent.EventType
	res.From = nftEvent.Source
	res.To = nftEvent.Target
	res.Price = nftEvent.Price
	//listen events
	// ants.Submit(func() {
	// 	aa(filehash, to, data, nftEvent, nft)
	// })
	aa(to, data, nftEvent, nft)
	res.State = nftEvent.State
	res.Date = nftEvent.EndDate
	return res, nil
}

func aa(to string, data TxData, nftEvent *model.Activity, nft *model.VideoMetadata) {
	logger.Info("[Nft purchase service] entry async listening...")
	var (
		txhash string
		err    error
	)
	if data.IsSent {
		txhash = data.Data
	} else {
		txhash, err = ctx.ChainClient.SendTx1(data.Data)
		if err != nil {
			nftEvent.State = model.FAILED.String()
			nftEvent.Update(ctx.GormDb)
			return
		}
	}
	//tx success,update activity state
	nftEvent.State = model.SUCCESS.String()
	nftEvent.TxHash = txhash
	if nftEvent.Update(ctx.GormDb) != nil {
		return
	}
	//change nft state
	nft.NftStatus = model.MINT.String()
	nft.Owner = to
	nft.Price = model.NULL
	//unexpected error,rollback
	if nft.Update(ctx.GormDb) != nil {
		nftEvent.State = model.FAILED.String()
		nftEvent.Update(ctx.GormDb)
	}
	logger.Info("[Nft purchase service] exit async listening")
}

func UpdateForTransfer(filehash, to string, data TxData) (dto.EventResp, error) {
	//query metadata
	var res dto.EventResp
	nft := &model.VideoMetadata{FileHash: filehash}
	resp, err := nft.Get(ctx.GormDb)
	if err != nil || len(resp) != 1 {
		return res, errors.New("query nft metadata error")
	}
	nft = &resp[0]
	if nft.NftStatus != model.MINT.String() {
		return res, errors.New("nft is not mint state")
	}
	if to == nft.Owner {
		return res, errors.New("cannot transfer your nft to yourself")
	}
	//create activity
	nftEvent := &model.Activity{
		EventType: model.ACT_TS.String(),
		Creator:   nft.Owner,
		Source:    nft.Owner,
		Target:    to,
		FileHash:  filehash,
		State:     model.LISTENING.String(),
		NftToken:  nft.NftToken,
		Price:     model.NULL,
		StartDate: time.Now().Local().Format(ctx.Time_FMT),
	}
	err = nftEvent.Create(ctx.GormDb)
	if err != nil {
		return res, errors.Wrap(err, "create transfer activity error")
	}

	nftEvent.EndDate = time.Now().Local().Format(ctx.Time_FMT)
	res.EventType = nftEvent.EventType
	res.From = nftEvent.Source
	res.To = nftEvent.Target
	//listen events
	// ants.Submit(func() {
	// 	bb(filehash, to, data, nftEvent, nft)
	// })
	bb(to, data, nftEvent, nft)
	res.State = nftEvent.State
	res.Date = nftEvent.EndDate
	return res, nil
}

func bb(to string, data TxData, nftEvent *model.Activity, nft *model.VideoMetadata) {
	logger.Info("[Nft transfer service] entry async listening...")
	var (
		txhash string
		err    error
	)
	if data.IsSent {
		txhash = data.Data
	} else {
		txhash, err = ctx.ChainClient.SendTx1(data.Data)
		if err != nil {
			nftEvent.State = model.FAILED.String()
			nftEvent.Update(ctx.GormDb)
			return
		}
	}
	//tx success,update activity state
	nftEvent.State = model.SUCCESS.String()
	nftEvent.TxHash = txhash
	if nftEvent.Update(ctx.GormDb) != nil {
		return
	}
	//change nft state
	nft.NftStatus = model.MINT.String()
	nft.Owner = to
	nft.Price = model.NULL
	//unexpected error,rollback
	if nft.Update(ctx.GormDb) != nil {
		nftEvent.State = model.FAILED.String()
		nftEvent.Update(ctx.GormDb)
	}
	logger.Info("[Nft transfer service] exit async listening")
}

func ChangeStatus(filehash, status, price string, data TxData) (dto.EventResp, error) {
	//query metadata
	var res dto.EventResp
	nft := &model.VideoMetadata{FileHash: filehash}
	resp, err := nft.Get(ctx.GormDb)
	if err != nil || len(resp) != 1 {
		return res, errors.New("query nft metadata error")
	}
	nft = &resp[0]
	//check
	status = strings.ToLower(status)
	if status != "list" && status != "unlist" {
		return res, errors.New("status error")
	}
	if status == "list" {
		status = model.LIST.String()
		if p, err := strconv.ParseFloat(price, 64); err != nil || p < 0 {
			return res, errors.Wrap(err, "invalid price")
		}
	} else {
		status = model.MINT.String()
	}
	if nft.NftStatus == status {
		return res, errors.New("status error,nft already list or unlist")
	}
	//create activity
	nftEvent := &model.Activity{
		EventType: model.ACT_ALT.String(),
		Creator:   nft.Creator,
		Source:    nft.NftStatus,
		Target:    status,
		FileHash:  filehash,
		State:     model.LISTENING.String(),
		NftToken:  nft.NftToken,
		Price:     price,
		StartDate: time.Now().Local().Format(ctx.Time_FMT),
	}
	err = nftEvent.Create(ctx.GormDb)
	if err != nil {
		return res, errors.Wrap(err, "create change status activity error")
	}

	nftEvent.EndDate = time.Now().Local().Format(ctx.Time_FMT)
	res.EventType = nftEvent.EventType
	res.From = nftEvent.Source
	res.To = nftEvent.Target
	res.Price = price
	//listen events
	//tx failed
	// ants.Submit(func() {
	// 	cc(filehash, status, price, data, nftEvent, nft)
	// })
	cc(status, price, data, nftEvent, nft)
	res.State = nftEvent.State
	res.Date = nftEvent.EndDate
	return res, nil
}

func cc(status, price string, data TxData, nftEvent *model.Activity, nft *model.VideoMetadata) {
	logger.Info("[Nft change status service] entry async listening...")
	var (
		txhash string
		err    error
	)
	if data.IsSent {
		txhash = data.Data
	} else {
		txhash, err = ctx.ChainClient.SendTx1(data.Data)
		if err != nil {
			nftEvent.State = model.FAILED.String()
			nftEvent.Update(ctx.GormDb)
			return
		}
	}
	//tx success,update activity state
	nftEvent.State = model.SUCCESS.String()
	nftEvent.TxHash = txhash
	if nftEvent.Update(ctx.GormDb) != nil {
		return
	}
	//change nft state
	nft.NftStatus = status
	nft.Price = price
	//unexpected error,rollback
	if nft.Update(ctx.GormDb) != nil {
		nftEvent.State = model.FAILED.String()
		nftEvent.Update(ctx.GormDb)
	}
	logger.Info("[Nft change status service] exit async listening")
}

func ChangePrice(filehash, price string, data TxData) (dto.EventResp, error) {
	//query metadata
	var res dto.EventResp
	nft := &model.VideoMetadata{FileHash: filehash}
	resp, err := nft.Get(ctx.GormDb)
	if err != nil || len(resp) != 1 {
		return res, errors.New("query nft metadata error")
	}
	nft = &resp[0]
	//check
	if price == nft.Price {
		return res, errors.New("price is the same as before")
	}
	if nft.NftStatus != model.LIST.String() {
		return res, errors.New("status error,nft not list")
	}
	//create activity
	nftEvent := &model.Activity{
		EventType: model.ACT_ALT.String(),
		Creator:   nft.Creator,
		Source:    nft.Price,
		Target:    price,
		FileHash:  filehash,
		State:     model.LISTENING.String(),
		NftToken:  nft.NftToken,
		Price:     price,
		StartDate: time.Now().Local().Format(ctx.Time_FMT),
	}
	err = nftEvent.Create(ctx.GormDb)
	if err != nil {
		return res, errors.Wrap(err, "create change price activity error")
	}

	nftEvent.EndDate = time.Now().Local().Format(ctx.Time_FMT)
	res.EventType = nftEvent.EventType
	res.From = nftEvent.Source
	res.To = nftEvent.Target
	res.Price = price
	//listen events
	// ants.Submit(func() {
	// 	dd(price, data, nftEvent, nft)
	// })
	dd(price, data, nftEvent, nft)
	res.State = nftEvent.State
	res.Date = nftEvent.EndDate
	return res, nil
}

func dd(price string, data TxData, nftEvent *model.Activity, nft *model.VideoMetadata) {
	logger.Info("[Nft change price service] entry async listening...")
	var (
		txhash string
		err    error
	)
	if data.IsSent {
		txhash = data.Data
	} else {
		txhash, err = ctx.ChainClient.SendTx1(data.Data)
		if err != nil {
			nftEvent.State = model.FAILED.String()
			nftEvent.Update(ctx.GormDb)
			return
		}
	}
	//tx success,update activity state
	nftEvent.State = model.SUCCESS.String()
	nftEvent.TxHash = txhash
	if nftEvent.Update(ctx.GormDb) != nil {
		return
	}
	//change nft state
	nft.Price = price
	//unexpected error,rollback
	if nft.Update(ctx.GormDb) != nil {
		nftEvent.State = model.FAILED.String()
		nftEvent.Update(ctx.GormDb)
	}
	logger.Info("[Nft change price service] exit async listening")
}
