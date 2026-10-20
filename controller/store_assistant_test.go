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
package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssistantEndpointAppendsOpenAIVersionOnce(t *testing.T) {
	cases := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{name: "host only", baseURL: "http://127.0.0.1:7863", expected: "http://127.0.0.1:7863/v1/chat/completions"},
		{name: "version present", baseURL: "https://api.example.com/v1", expected: "https://api.example.com/v1/chat/completions"},
		{name: "trailing slash", baseURL: "https://api.example.com/v1/", expected: "https://api.example.com/v1/chat/completions"},
		{name: "host with trailing slash", baseURL: "https://api.example.com/", expected: "https://api.example.com/v1/chat/completions"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, assistantEndpoint(testCase.baseURL))
		})
	}
}
