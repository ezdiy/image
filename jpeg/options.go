package jpeg

import "image"

type Options struct {
	Quality 	int		// Encoding quality 1-100
	*image.Rectangle	// Save (crop) only this rectangle.
	Progressive bool	// Code for progressive ("blurry first") loading.
	Fast 		bool	// Disable size optimizations, makes saving faster.
}

