package engine

import "discoveryx/internal/model"

func stageState(name string) model.StageState { return model.StageState{Name: name, Status: "Pending"} }
