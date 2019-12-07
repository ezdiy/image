package jpeg

import (
	"image"
	"image/color"
)

// Options for encoder.
type Options struct {
	// Basic settings.
	Quality             int // Encoding quality 1-100. 0 is special value defaulting to 75.
	ForceBaseline       bool
	NoProgressive       bool  // Don't encode multiple scans.
	FastHufftab         bool    // Disable huff table optimizations, makes saving faster.
	NoFancyDownsampling bool    // When saving RGB into YCbCr with subsampled chroma
	SmoothingFactor     int     // Blur input 0-100%, to smooth out ringing around edges.
	Gamma               float64 // Gamma correction for input
	DCTMethod

	// Extended settings via GUID table
	Ext ExtOptions

	// Obscure features if you know what you're doing.
	QuantTables      *[2][64]byte // Will get linearly scaled by quality.
	ArithmeticCoding bool         // Enable arithmetic coding. Poorly supported.

	NBWritten *int // If not nil, stores number of bytes written
}

type ExtOptions map[uint64]interface{}
type DCTMethod int

const (
	// For DCTMethod
	DCTISlow DCTMethod = iota
	DCTIFast
	DCTFloat

	// GUIDs set as a key for Options.Ext[] map.
	// There's 4 bits in front doing a ghetto runtime type check.
	//
	// Boolean
	OptScans           = 0x0680C061E // optimize progressive coding scan
	OptTrellis         = 0x0C5122033 // use trellis quantization
	OptTrellisDC       = 0x0339D4C0C // use trellis quant for DC coefficient
	OptEOB             = 0x0D7F73780 // optimize for sequences of EOB
	OptLambdaWeight    = 0x0339DB65F // use lambda weighting table
	OptTrellisUseScans = 0x0FD841435 // use scans in trellis optimization
	OptTrellisQ        = 0x0E12AE269 // optimize quant table in trellis loop
	OptDeringing       = 0x03F4BBBF9 // preprocess input to reduce ringing of edges on white background
	//
	// Float
	OptLambdaLogScale1      = 0x15B61A599
	OptLambdaLogScale2      = 0x1B9BBAE03
	OPtTrellisDeltaDCWeight = 0x113775453
	//
	// Integer
	OptCompressProfile   = 0x2E9918625 // compression profile
	OptTrellisFreqSplit  = 0x26FAFF127 // splitting point for frequency in trellis quantization
	OptTrellisLoops      = 0x2B63EBF39 // number of trellis loops
	OptBaseQuantTblIndex = 0x244492AB1 // base quantization table index
	OptDCScanMode        = 0x20BE7AD3C // DC scan optimization mode

	// Profiles for OptCompressProfile
	ProfileMaxCompression = 0x5D083AAD
	ProfileFastest        = 0x2AEA5CB4
)

var (

	// WhitelistedSubsampling decoder option default.
	// The library supports all ratios image.YCbCr knows about,
	// however it's usually not a good idea to decode into those.
	// Unlisted ratios are upsacled into RGB or Gray colorspace.
	SaneSubsampling = []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio420, // Chroma halved H/V, most JPG files.
		image.YCbCrSubsampleRatio444, // Full chroma, HD files.
	}

	// Colorspace default is "give me what's actually inside the file".
	// If the source file comes in color space not listed, coercion will be
	// attempted to nearest one (doesn't work for CMYK).
	AllColorspaces = []color.Model{
		color.GrayModel,
		color.RGBAModel,
		color.YCbCrModel,
		color.CMYKModel,
	}

	// Default decoder options if none are specified, as well for compat API.
	DefaultDecoderOptions = DecoderOptions{
		OutputColorspaces:      AllColorspaces,
		WhitelistedSubsampling: SaneSubsampling,
	}

	// Default encoder options if none are specified, as well for compat API.
	DefaultEncoderOptions = Options{}
)

type DecoderOptions struct {
	DCTMethod
	// The library will coerce the source color model to a nearest one
	// listed when actual source JPG File colorspace is not on the list.
	//
	// File->Image possible coercions
	//
	// YCbCr->YCbCr (trumps conversions, but only if subsampling rate allowed)
	// Gray->Gray (trumps conversions)
	// RGB->RGB (trumps conversions)
	// RBG->Gray (Gray listed, RGB not)
	// YCbCr->Gray (No YCbCr, no RGB, but Gray listed)
	// Gray->RGB (Gray not listed, RGB is)
	// YCbCr->RGB (YCbCr not listed, RGB is)
	// CMYK/YCCK/YCbCrK->CMYK (forced conversion, and only to CMYK)
	//
	// If empty/nil, will enable all color spaces.
	OutputColorspaces []color.Model

	// In case the file comes in with subsampling ratio not on the list, it will
	// be treated as if YCbCr wasn't in OutputColorspaces, and chroma will get
	// upscaled inside RGB or Gray.
	//
	// If empty/nil, enables all ratios.
	WhitelistedSubsampling []image.YCbCrSubsampleRatio

	// Disable raw decoding for image.Gray. Can help with especially broken files.
	// To disable raw decoding for YCbCr (forces RGB), remove it from OutputColorspaces.
	NoRawDecodingGray bool

	// Setting these to true makes decoding a bit faster and uglier
	NoFancyUpsampling bool
	NoBlockSmoothing  bool

	// If this pointer is set, will be filled by image information about color space
	// and dimensions. No actual decoding will be done.
	*image.Config

	// If not nil, filled with number of bytes read from the input stream.
	NBRead *int
}

// Check if output is allowed in a given colorspace.
func (d *DecoderOptions) HasModel(c color.Model) (r bool) {
	if len(d.OutputColorspaces) == 0 {
		return true
	}
	for _, v := range d.OutputColorspaces {
		if v == c {
			r = true
		}
	}
	return
}

// Check if given ratio is whitelisted.
func (d *DecoderOptions) HasSSR(c image.YCbCrSubsampleRatio) (r bool) {
	if len(d.WhitelistedSubsampling) == 0 {
		return true
	}
	for _, v := range d.WhitelistedSubsampling {
		if v == c {
			r = true
		}
	}
	return
}
