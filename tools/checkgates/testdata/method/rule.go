// SPDX-FileCopyrightText: 2026 randyinthedev
// SPDX-License-Identifier: Apache-2.0

package scope

type Master struct{}

// GATE: 배선 필수
func (m *Master) ClassifyObserved(id string) bool { return false }
