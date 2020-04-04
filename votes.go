// Copyright 2020 Fabian Wenzelmann <fabianwen@posteo.eu>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gopolls

import (
	"encoding/csv"
	"io"
)

// CSV //

type VotesCSVWriter struct {
	csv *csv.Writer
}

func NewVotesCSVWriter(w io.Writer) *VotesCSVWriter {
	return &VotesCSVWriter{csv: csv.NewWriter(w)}
}

func (writer *VotesCSVWriter) writeCSVHead(skels []AbstractPollSkeleton) error {
	row := make([]string, len(skels)+1)
	row[0] = "voter"
	for i, skel := range skels {
		row[i+1] = skel.GetName()
	}
	return writer.csv.Write(row)
}

func (writer *VotesCSVWriter) writeEmptyRecords(voters []*Voter, skels []AbstractPollSkeleton) error {
	// row will be re-used
	row := make([]string, len(skels)+1)
	for _, voter := range voters {
		row[0] = voter.Name
		for j, skel := range skels {
			row[j+1] = skel.GetName()
		}
		if err := writer.csv.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func (writer *VotesCSVWriter) GenerateEmptyTemplate(voters []*Voter, skels []AbstractPollSkeleton) error {
	if err := writer.writeCSVHead(skels); err != nil {
		return err
	}
	if err := writer.writeEmptyRecords(voters, skels); err != nil {
		return err
	}
	writer.csv.Flush()
	return writer.csv.Error()
}
