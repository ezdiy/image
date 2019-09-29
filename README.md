### Some additional image format bindings for Go

* mozjpeg bindings (or libjpeg, turbo).
  Exposes extensive knobs for compression settings.
  This is geared for high-concurrency transcoding servers.
  Soft-fails w/o cgo by wrapping std Go jpg codec.
* openjpeg (JPEG2000). Used for maps and medical imaging. Very slow, but quality can even surpass webp.
  cgo mandatory.

Other formats are out there. Consult imports in
[this demo application](https://github.com/ezdiy/image/blob/master/cmd/imgconv/main.go).
