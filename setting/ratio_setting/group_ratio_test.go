/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckGroupColor(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		wantErr bool
	}{
		{name: "empty object", jsonStr: `{}`, wantErr: false},
		{name: "single valid hex", jsonStr: `{"default":"#1a2b3c"}`, wantErr: false},
		{name: "uppercase hex", jsonStr: `{"vip":"#ABCDEF"}`, wantErr: false},
		{name: "mixed hex", jsonStr: `{"default":"#1AbC3f"}`, wantErr: false},
		{name: "multiple groups", jsonStr: `{"default":"#112233","vip":"#445566"}`, wantErr: false},
		{name: "empty value resets to auto", jsonStr: `{"vip":""}`, wantErr: false},
		{name: "null value means auto", jsonStr: `{"vip":null}`, wantErr: false},
		{name: "not a color name", jsonStr: `{"vip":"red"}`, wantErr: true},
		{name: "too short hex", jsonStr: `{"vip":"#123"}`, wantErr: true},
		{name: "non-hex digit", jsonStr: `{"vip":"#gggggg"}`, wantErr: true},
		{name: "missing hash prefix", jsonStr: `{"vip":"123456"}`, wantErr: true},
		{name: "array instead of object", jsonStr: `["#112233"]`, wantErr: true},
		{name: "number value", jsonStr: `{"vip":123}`, wantErr: true},
		{name: "malformed json", jsonStr: `{"vip":`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckGroupColor(tt.jsonStr)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUpdateGroupColorByJSONString(t *testing.T) {
	require.NoError(t, UpdateGroupColorByJSONString(`{"default":"#112233","vip":"#445566"}`))

	colors := GetGroupColorCopy()
	assert.Equal(t, "#112233", colors["default"])
	assert.Equal(t, "#445566", colors["vip"])

	// Updating with a fresh map replaces all keys (empty value marks auto).
	require.NoError(t, UpdateGroupColorByJSONString(`{"vip":""}`))
	colors = GetGroupColorCopy()
	assert.NotContains(t, colors, "default")
	assert.Equal(t, "", colors["vip"])

	// Invalid input must not mutate the map.
	require.Error(t, UpdateGroupColorByJSONString(`not-json`))
	colors = GetGroupColorCopy()
	assert.Equal(t, "", colors["vip"])
}
