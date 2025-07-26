package penelope

func (p *Penelope) addUserMemory(did, memory string) error {
	dbMemory := &UserMemory{
		Rkey:   p.clock.Next().String(),
		Did:    did,
		Memory: memory,
	}
	if err := p.db.Create(dbMemory).Error; err != nil {
		return err
	}
	return nil
}

func (p *Penelope) getUserMemory(did string) (string, error) {
	var dbMemories []UserMemory
	if err := p.db.Raw("SELECT * FROM user_memories WHERE did = ?", did).Scan(&dbMemories).Error; err != nil {
		return "", err
	}

	memories := ""
	for _, m := range dbMemories {
		memories += m.Memory + "\n"
	}

	return memories, nil
}
