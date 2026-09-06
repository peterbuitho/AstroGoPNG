package xisf

import "fmt"

// lz4DecompressBlock decodes an LZ4 block (no frame header) into dst, which must
// be exactly the uncompressed size. XISF's "lz4" and "lz4hc" codecs both use
// this block format.
func lz4DecompressBlock(src, dst []byte) error {
	sp, dp := 0, 0
	for sp < len(src) {
		token := src[sp]
		sp++

		litLen := int(token >> 4)
		if litLen == 15 {
			for {
				if sp >= len(src) {
					return fmt.Errorf("lz4: truncated literal length")
				}
				b := src[sp]
				sp++
				litLen += int(b)
				if b != 255 {
					break
				}
			}
		}
		if sp+litLen > len(src) || dp+litLen > len(dst) {
			return fmt.Errorf("lz4: literal run out of range")
		}
		copy(dst[dp:dp+litLen], src[sp:sp+litLen])
		sp += litLen
		dp += litLen

		if sp == len(src) {
			break // last sequence is literals only
		}
		if sp+2 > len(src) {
			return fmt.Errorf("lz4: truncated match offset")
		}
		offset := int(src[sp]) | int(src[sp+1])<<8
		sp += 2
		if offset == 0 || offset > dp {
			return fmt.Errorf("lz4: bad match offset %d", offset)
		}

		matchLen := int(token & 0x0f)
		if matchLen == 15 {
			for {
				if sp >= len(src) {
					return fmt.Errorf("lz4: truncated match length")
				}
				b := src[sp]
				sp++
				matchLen += int(b)
				if b != 255 {
					break
				}
			}
		}
		matchLen += 4 // minmatch

		if dp+matchLen > len(dst) {
			return fmt.Errorf("lz4: match run past end")
		}
		from := dp - offset
		for i := 0; i < matchLen; i++ { // byte-by-byte: ranges may overlap
			dst[dp+i] = dst[from+i]
		}
		dp += matchLen
	}
	if dp != len(dst) {
		return fmt.Errorf("lz4: produced %d bytes, expected %d", dp, len(dst))
	}
	return nil
}
