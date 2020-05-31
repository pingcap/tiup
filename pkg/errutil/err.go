// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package errutil

import "github.com/joomcode/errorx"

var (
	// ErrPropSuggestion is a property of an Error that will be printed as the suggestion.
	ErrPropSuggestion = errorx.RegisterProperty("suggestion")

	// ErrTraitPreCheck means that the Error is a pre-check error so that no error logs will be outputted directly.
	ErrTraitPreCheck = errorx.RegisterTrait("pre_check")
)
