package rep

import (
	"fmt"
	"regexp"
	"strings"

	"philosopher/lib/dat"
	"philosopher/lib/id"
)

// PeptideMap struct
type PeptideMap struct {
	Sequence  string
	IonForm   string
	Protein   string
	ProteinID string
	Gene      string
	Proteins  map[string]int
}

// UpdateNumberOfEnzymaticTermini collects the NTT from ProteinProphet
// and passes along to the final Protein structure.
func (evi *Evidence) UpdateNumberOfEnzymaticTermini() {

	// restore the original prot.xml output
	var p id.ProtIDList
	p.Restore()

	// collect the updated ntt for each peptide-protein pair
	var nttPeptidetoProptein = make(map[string]uint8)

	for _, i := range p {
		for _, j := range i.PeptideIons {
			if !strings.Contains(i.ProteinName, "rev_") {
				key := fmt.Sprintf("%s#%s", j.PeptideSequence, i.ProteinName)
				nttPeptidetoProptein[key] = j.NumberOfEnzymaticTermini
			}
		}
	}

	for i := range evi.PSM {

		key := fmt.Sprintf("%s#%s", evi.PSM[i].Peptide, evi.PSM[i].Protein)
		ntt, ok := nttPeptidetoProptein[key]
		if ok {
			evi.PSM[i].NumberOfEnzymaticTermini = int(ntt)
		}
	}

}

// UpdateIonStatus pushes back to ion and psm evideces the uniqueness and razorness status of each peptide and ion
func (evi *Evidence) UpdateIonStatus(decoyTag string) {

	var uniqueMap = make(map[string]bool)
	var urazorMap = make(map[string]string)
	var sequenceMap = make(map[string]string)
	var uniqueSeqMap = make(map[string]string)

	for _, i := range evi.Proteins {

		for _, j := range i.TotalPeptideIons {
			if j.IsUnique {
				uniqueMap[j.IonForm] = true
			}
		}

		for _, j := range i.TotalPeptideIons {
			if j.IsURazor {
				urazorMap[j.IonForm] = i.PartHeader
				sequenceMap[j.Sequence] = i.PartHeader
			}
		}
	}

	for i := range evi.PSM {
		// the decoy tag checking is a failsafe mechanism to avoid proteins
		// with real complex razor case decisions to pass downstream
		// wrong classifications. If by any chance the protein gets assigned to
		// a razor decoy, this mechanism avoids the replacement

		rp, rOK := urazorMap[evi.PSM[i].IonForm]
		if rOK {

			evi.PSM[i].IsURazor = true

			// we found cases where the peptide maps to both target and decoy but is
			// assigned as razor to the decoy. the IF statement below replaces the
			// decoy by the target but it was removed because in some cases the protein
			// does not pass the FDR filtering.

			evi.PSM[i].MappedProteins[evi.PSM[i].Protein] = 0
			delete(evi.PSM[i].MappedProteins, rp)
			evi.PSM[i].Protein = rp

			// if strings.Contains(rp, decoyTag) {
			// 	evi.PSM[i].IsDecoy = true
			// } else {
			// 	evi.PSM[i].IsDecoy = false
			// }
		}

		if !evi.PSM[i].IsURazor {
			sp, sOK := sequenceMap[evi.PSM[i].Peptide]
			if sOK {

				evi.PSM[i].IsURazor = true

				// we found cases where the peptide maps to both target and decoy but is
				// assigned as razor to the decoy. the IF statement below replaces the
				// decoy by the target but it was removed because in some cases the protein
				// does not pass the FDR filtering.

				evi.PSM[i].MappedProteins[evi.PSM[i].Protein] = 0
				delete(evi.PSM[i].MappedProteins, sp)
				evi.PSM[i].Protein = sp

				if strings.Contains(sp, decoyTag) {
					evi.PSM[i].IsDecoy = true
				}
			}

			_, uOK := uniqueMap[evi.PSM[i].IonForm]
			if uOK {
				evi.PSM[i].IsUnique = true
			}

			uniqueSeqMap[evi.PSM[i].Peptide] = evi.PSM[i].Protein
		}
	}

	for i := range evi.Ions {
		rp, rOK := urazorMap[evi.Ions[i].IonForm]
		if rOK {

			evi.Ions[i].IsURazor = true

			evi.Ions[i].MappedProteins[evi.Ions[i].Protein] = 0
			delete(evi.Ions[i].MappedProteins, rp)
			evi.Ions[i].Protein = rp

			if strings.Contains(rp, decoyTag) {
				evi.Ions[i].IsDecoy = true
			}

		}
		_, uOK := uniqueMap[evi.Ions[i].IonForm]
		if uOK {
			evi.Ions[i].IsUnique = true
		} else {
			evi.Ions[i].IsUnique = false
		}
	}

	for i := range evi.Peptides {
		v, ok := sequenceMap[evi.Peptides[i].Sequence]
		if ok {
			evi.Peptides[i].MappedProteins[evi.Peptides[i].Protein] = 0
			delete(evi.Peptides[i].MappedProteins, v)
			evi.Peptides[i].Protein = v
		}

		if strings.Contains(v, decoyTag) {
			evi.Peptides[i].IsDecoy = true
		}

	}

}

// UpdateIonModCount counts how many times each ion is observed modified and not modified
func (evi *Evidence) UpdateIonModCount() {

	// recreate the ion list from the main report object
	var AllIons = make(map[string]int)
	var ModIons = make(map[string]int)
	var UnModIons = make(map[string]int)

	for _, i := range evi.Ions {
		AllIons[i.IonForm] = 0
		ModIons[i.IonForm] = 0
		UnModIons[i.IonForm] = 0
	}

	// range over PSMs looking for modified and not modified evidences
	// if they exist on the ions map, get the numbers
	for _, i := range evi.PSM {

		// check the map
		_, ok := AllIons[i.IonForm]
		if ok {

			if i.Massdiff >= -0.99 && i.Massdiff <= 0.99 {
				UnModIons[i.IonForm]++
			} else {
				ModIons[i.IonForm]++
			}

		}
	}

}

// SyncPSMToProteins forces the synchronization between the filtered proteins, and the remaining structures.
func (evi *Evidence) SyncPSMToProteins() {

	var proteinIndex = make(map[string]uint8)
	var newPSM PSMEvidenceList
	var newIons IonEvidenceList
	var newPeptides PeptideEvidenceList

	for _, i := range evi.Proteins {
		//if !i.IsDecoy {
		proteinIndex[i.ProteinID] = 0
		//}
	}

	for _, i := range evi.PSM {
		_, ok := proteinIndex[i.ProteinID]
		if ok {
			newPSM = append(newPSM, i)
		}
	}
	evi.PSM = newPSM

	for _, i := range evi.Ions {
		_, ok := proteinIndex[i.ProteinID]
		if ok {
			newIons = append(newIons, i)
		}
	}
	evi.Ions = newIons

	for _, i := range evi.Peptides {
		_, ok := proteinIndex[i.ProteinID]
		if ok {
			newPeptides = append(newPeptides, i)
		}
	}
	evi.Peptides = newPeptides

}

// UpdateLayerswithDatabase will fix the protein and gene assignments based on the database data
func (evi *Evidence) UpdateLayerswithDatabase(decoyTag string) {

	var dtb dat.Base
	dtb.Restore()

	var proteinIDMap = make(map[string]string)
	var entryNameMap = make(map[string]string)
	var geneMap = make(map[string]string)
	var descriptionMap = make(map[string]string)
	var sequenceMap = make(map[string]string)
	var pepPrevAA = make(map[string]string)
	var pepNextAA = make(map[string]string)

	for _, j := range dtb.Records {
		if !j.IsDecoy {
			proteinIDMap[j.PartHeader] = j.ID
			entryNameMap[j.PartHeader] = j.EntryName
			geneMap[j.PartHeader] = j.GeneNames
			descriptionMap[j.PartHeader] = j.Description
			sequenceMap[j.PartHeader] = j.Sequence
		}
	}

	for i := range evi.PSM {

		id := evi.PSM[i].Protein
		if evi.PSM[i].IsDecoy {
			id = strings.Replace(id, decoyTag, "", 1)
		}

		evi.PSM[i].ProteinID = proteinIDMap[id]
		evi.PSM[i].EntryName = entryNameMap[id]
		evi.PSM[i].GeneName = geneMap[id]
		evi.PSM[i].ProteinDescription = descriptionMap[id]

		// update mapped genes
		for k := range evi.PSM[i].MappedProteins {
			if !strings.Contains(k, decoyTag) {
				evi.PSM[i].MappedGenes[geneMap[k]] = 0
			}
		}

		// map the peptide to the protein
		re := regexp.MustCompile(evi.PSM[i].Peptide)
		reMatch := re.FindStringIndex(sequenceMap[id])
		if len(reMatch) > 0 {

			evi.PSM[i].ProteinStart = reMatch[0]
			evi.PSM[i].ProteinEnd = reMatch[1]

			if (reMatch[0]) <= 0 {
				evi.PSM[i].PrevAA = string(sequenceMap[id][0])
			} else {
				evi.PSM[i].PrevAA = string(sequenceMap[id][reMatch[0]-1])
			}

			if (reMatch[1] + 1) >= len(sequenceMap[id]) {
				evi.PSM[i].NextAA = string(sequenceMap[id][len(sequenceMap[id])-1])
			} else {
				evi.PSM[i].NextAA = string(sequenceMap[id][reMatch[1]])
			}

		} else {

			var peptide string

			if strings.Contains(evi.PSM[i].Peptide, "I") {
				peptide = strings.Replace(evi.PSM[i].Peptide, "I", "L", -1)
			}
			if strings.Contains(evi.PSM[i].Peptide, "L") {
				peptide = strings.Replace(evi.PSM[i].Peptide, "L", "I", -1)
			}

			re := regexp.MustCompile(peptide)
			reMatch := re.FindStringIndex(sequenceMap[id])
			if len(reMatch) > 0 {
				evi.PSM[i].ProteinStart = reMatch[0]
				evi.PSM[i].ProteinEnd = reMatch[1]

				if (reMatch[0]) <= 0 {
					evi.PSM[i].PrevAA = string(sequenceMap[id][0])
				} else {
					evi.PSM[i].PrevAA = string(sequenceMap[id][reMatch[0]-1])
				}

				if (reMatch[1] + 1) >= len(sequenceMap[id]) {
					evi.PSM[i].NextAA = string(sequenceMap[id][len(sequenceMap[id])-1])
				} else {
					evi.PSM[i].NextAA = string(sequenceMap[id][reMatch[1]])
				}
			}
		}

		pepPrevAA[evi.PSM[i].Peptide] = evi.PSM[i].PrevAA
		pepNextAA[evi.PSM[i].Peptide] = evi.PSM[i].NextAA
	}

	for i := range evi.Ions {

		id := evi.Ions[i].Protein
		if evi.Ions[i].IsDecoy {
			id = strings.Replace(id, decoyTag, "", 1)
		}

		evi.Ions[i].ProteinID = proteinIDMap[id]
		evi.Ions[i].EntryName = entryNameMap[id]
		evi.Ions[i].GeneName = geneMap[id]
		evi.Ions[i].ProteinDescription = descriptionMap[id]

		// update mapped genes
		for k := range evi.Ions[i].MappedProteins {
			if !strings.Contains(k, decoyTag) {
				evi.Ions[i].MappedGenes[geneMap[k]] = 0
			}
		}

		evi.Ions[i].PrevAA = pepPrevAA[evi.Ions[i].Sequence]
		evi.Ions[i].NextAA = pepNextAA[evi.Ions[i].Sequence]
	}

	for i := range evi.Peptides {

		id := evi.Peptides[i].Protein
		if evi.Peptides[i].IsDecoy {
			id = strings.Replace(id, decoyTag, "", 1)
		}

		evi.Peptides[i].ProteinID = proteinIDMap[id]
		evi.Peptides[i].EntryName = entryNameMap[id]
		evi.Peptides[i].GeneName = geneMap[id]
		evi.Peptides[i].ProteinDescription = descriptionMap[id]

		// update mapped genes
		for k := range evi.Peptides[i].MappedProteins {
			if !strings.Contains(k, decoyTag) {
				evi.Peptides[i].MappedGenes[geneMap[k]] = 0
			}
		}

		evi.Peptides[i].PrevAA = pepPrevAA[evi.Peptides[i].Sequence]
		evi.Peptides[i].NextAA = pepNextAA[evi.Peptides[i].Sequence]
	}

}

// UpdateSupportingSpectra pushes back from PSM to Protein the new supporting spectra from razor results
func (evi *Evidence) UpdateSupportingSpectra() {

	var ptSupSpec = make(map[string][]string)
	var uniqueSpec = make(map[string][]string)
	var razorSpec = make(map[string][]string)

	for _, i := range evi.PSM {

		_, ok := ptSupSpec[i.Protein]
		if !ok {
			ptSupSpec[i.Protein] = append(ptSupSpec[i.Protein], i.Spectrum)
		} else {
			ptSupSpec[i.Protein] = append(ptSupSpec[i.Protein], i.Spectrum)
		}

		if i.IsUnique {
			_, ok := uniqueSpec[i.IonForm]
			if !ok {
				uniqueSpec[i.IonForm] = append(uniqueSpec[i.IonForm], i.Spectrum)
			} else {
				uniqueSpec[i.IonForm] = append(uniqueSpec[i.IonForm], i.Spectrum)
			}
		}

		if i.IsURazor {
			_, ok := razorSpec[i.IonForm]
			if !ok {
				razorSpec[i.IonForm] = append(razorSpec[i.IonForm], i.Spectrum)
			} else {
				razorSpec[i.IonForm] = append(razorSpec[i.IonForm], i.Spectrum)
			}
		}

	}

	for i := range evi.Proteins {
		for j := range evi.Proteins[i].TotalPeptideIons {

			if len(evi.Proteins[i].TotalPeptideIons[j].Spectra) == 0 {
				delete(evi.Proteins[i].TotalPeptideIons, j)
			}
		}
	}

	for i := range evi.Proteins {

		v, ok := ptSupSpec[evi.Proteins[i].PartHeader]
		if ok {
			for _, j := range v {
				evi.Proteins[i].SupportingSpectra[j] = 0
			}
		}

		for k := range evi.Proteins[i].TotalPeptideIons {

			Up, UOK := uniqueSpec[evi.Proteins[i].TotalPeptideIons[k].IonForm]
			if UOK && evi.Proteins[i].TotalPeptideIons[k].IsUnique {
				for _, l := range Up {
					evi.Proteins[i].TotalPeptideIons[k].Spectra[l] = 0
				}
			}

			Rp, ROK := razorSpec[evi.Proteins[i].TotalPeptideIons[k].IonForm]
			if ROK && evi.Proteins[i].TotalPeptideIons[k].IsURazor {
				for _, l := range Rp {
					evi.Proteins[i].TotalPeptideIons[k].Spectra[l] = 0
				}
			}

		}

	}

}

// UpdatePeptideModCount counts how many times each peptide is observed modified and not modified
func (evi *Evidence) UpdatePeptideModCount() {

	// recreate the ion list from the main report object
	var all = make(map[string]int)
	var mod = make(map[string]int)
	var unmod = make(map[string]int)

	for _, i := range evi.Peptides {
		all[i.Sequence] = 0
		mod[i.Sequence] = 0
		unmod[i.Sequence] = 0
	}

	// range over PSMs looking for modified and not modified evidences
	// if they exist on the ions map, get the numbers
	for _, i := range evi.PSM {

		_, ok := all[i.Peptide]
		if ok {

			if i.Massdiff >= -0.99 && i.Massdiff <= 0.99 {
				unmod[i.Peptide]++
			} else {
				mod[i.Peptide]++
			}

		}
	}

	for i := range evi.Peptides {

		v1, ok1 := unmod[evi.Peptides[i].Sequence]
		if ok1 {
			evi.Peptides[i].UnModifiedObservations = v1
		}

		v2, ok2 := mod[evi.Peptides[i].Sequence]
		if ok2 {
			evi.Peptides[i].ModifiedObservations = v2
		}

	}

}
