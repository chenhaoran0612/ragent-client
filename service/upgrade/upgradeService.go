package upgrade

type UpgradeService struct{}

func NewUpgradeService() *UpgradeService {

	upgradeService := new(UpgradeService)

	return upgradeService
}

func (upgradeService *UpgradeService) Upgrade() (err error) {

	return
}
