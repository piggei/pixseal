package watermark

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func testImage(w,h int)*image.NRGBA { img:=image.NewNRGBA(image.Rect(0,0,w,h)); for y:=0;y<h;y++ { for x:=0;x<w;x++ { img.SetNRGBA(x,y,color.NRGBA{uint8((x*3+y)%256),uint8((x+y*2)%256),uint8((x*2+y*3)%256),255}) } }; return img }
func TestRoundTrip(t *testing.T){ key:=[]byte("correct horse battery staple"); msg:=[]byte("hello robust world"); marked,err:=Embed(testImage(512,512),msg,key,DefaultOptions()); if err!=nil{t.Fatal(err)}; got,_,err:=Extract(marked,key,DefaultOptions()); if err!=nil{t.Fatal(err)}; if !bytes.Equal(got,msg){t.Fatalf("got %q",got)} }
func TestJPEG(t *testing.T){ key:=[]byte("correct horse battery staple"); msg:=[]byte("jpeg test"); marked,err:=Embed(testImage(512,512),msg,key,DefaultOptions()); if err!=nil{t.Fatal(err)}; var b bytes.Buffer; if err=jpeg.Encode(&b,marked,&jpeg.Options{Quality:82});err!=nil{t.Fatal(err)}; decoded,err:=jpeg.Decode(&b);if err!=nil{t.Fatal(err)}; got,_,err:=Extract(decoded,key,DefaultOptions());if err!=nil{t.Fatal(err)};if !bytes.Equal(got,msg){t.Fatalf("got %q",got)} }
