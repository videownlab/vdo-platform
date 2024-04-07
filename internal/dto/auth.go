package dto

type (
	EmailAuthCodeReq struct {
		Email string `json:"email" form:"email" valid:"email"`
	}

	EmailLoginReq struct {
		Email    string `json:"email" form:"email" binding:"required" valid:"email"`
		AuthCode string `json:"authCode" form:"authCode" binding:"required,min=1,max=6"`
	}

	DotWalletLoginReq struct {
		Address   string `binding:"required,min=40,max=60" json:"address" form:"address"`
		Timestamp int64  `binding:"required" json:"timestamp" form:"timestamp"`
		Sign      string `form:"sign" binding:"required" json:"sign"`
	}

	EthWalletLoginReq struct {
		EthAddress string `binding:"required,min=40,max=60" json:"ethAddress" form:"ethAddress"`
		DotAddress string `binding:"required,min=40,max=60" json:"dotAddress" form:"dotAddress"`
		Timestamp  int64  `binding:"required" json:"timestamp" form:"timestamp"`
		Sign       string `form:"sign" binding:"required" json:"sign"`
	}

	SignTxReq struct {
		WalletAddress string `binding:"required,min=40,max=60" json:"walletAddress" form:"walletAddress"`
		Extrinsic     string `binding:"required" json:"extrinsic" form:"extrinsic"`
	}
)
