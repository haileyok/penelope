package penelope

type Block struct {
	Did string `gorm:"uniqueIndex"`
	Id  string `gorm:"index"`
}
