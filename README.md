### Some additional image format bindings for Go

* mozjpeg bindings (or libjpeg, turbo).
  * Exposes extensive knobs for compression settings.
  * Supports true RGB/CMYK jpeg files as image.* equivalents, and fuzzy image format coercions.
  * Supports all image.YCbCr subsampling ratios.
  * This is geared for high-concurrency transcoding servers.
  * Soft-fails w/o cgo by wrapping std Go jpg codec.
* openjpeg (JPEG2000).
  * Used for maps and medical imaging.
  * Very slow, but quality can even surpass webp.
  * Pictures only, no tiling atm.
  * cgo mandatory.

Libraries for other formats are out there. Consult imports in
[this demo application](https://github.com/ezdiy/image/blob/master/cmd/imgconv/main.go).
