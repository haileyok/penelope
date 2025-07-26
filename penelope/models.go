package penelope

import "gorm.io/gorm"

type Block struct {
	Did string `gorm:"uniqueIndex"`
	Id  string `gorm:"index"`
}

type UserMemory struct {
	gorm.Model
	Rkey   string `gorm:"index"`
	Did    string `gorm:"index"`
	Memory string
}
