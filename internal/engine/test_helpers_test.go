package engine

import "github.com/naroSEC/DiscoveryX/internal/model"

func stageState(name string) model.StageState { return model.StageState{Name: name, Status: "Pending"} }
