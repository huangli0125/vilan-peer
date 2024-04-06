package common

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

// 加密解密接口
type Crypt interface { // 密码内置
	Encode(src []byte, dst []byte) (int, error)
	Decode(src []byte, dst []byte) (int, error)
}
type AesCrypt struct {
	keyByte   []byte
	block     cipher.Block
	blockSize int
}

// AES加密
func NewAesCrypt(key string) *AesCrypt {
	if len(key) == 0 {
		return nil
	}
	a := &AesCrypt{}
	data := []byte(key)
	k := len(data)
	keyLen := 16
	if k <= 16 {
		keyLen = 16
	} else if k <= 16 {
		keyLen = 24
	} else {
		keyLen = 32
	}
	a.keyByte = make([]byte, keyLen)
	copy(a.keyByte, data)
	block, err := aes.NewCipher(a.keyByte)
	if err != nil {
		return nil
	}
	a.block = block
	a.blockSize = block.BlockSize()
	return a
}

func (a *AesCrypt) Encode(src []byte, dst []byte) (int, error) {
	dataLen := len(src)
	padding := a.blockSize - dataLen%a.blockSize
	padData := bytes.Repeat([]byte{byte(padding)}, padding)
	src = append(src, padData...)
	if len(dst) < dataLen+padding {
		return 0, errors.New("dst buf too small")
	}
	blockMode := cipher.NewCBCEncrypter(a.block, a.keyByte)
	blockMode.CryptBlocks(dst, src) // 每次调用保留状态  所以每次都是新建
	return dataLen + padding, nil
}

func (a *AesCrypt) Decode(src []byte, dst []byte) (int, error) {
	defer func() {
		recover()
	}()
	srcSize := len(src)
	blockMode := cipher.NewCBCDecrypter(a.block, a.keyByte)
	blockMode.CryptBlocks(dst, src)
	unPadding := int(dst[srcSize-1])
	return srcSize - unPadding, nil
}

type DesCrypt struct {
	keyByte   []byte
	block     cipher.Block
	blockSize int
}

// DES加密
func NewDesCrypt(key string) *DesCrypt {
	if len(key) == 0 {
		return nil
	}
	d := &DesCrypt{}
	data := []byte(key)
	d.keyByte = make([]byte, 8)
	copy(d.keyByte, data)
	block, err := des.NewCipher(d.keyByte)
	if err != nil {
		return nil
	}
	d.block = block
	d.blockSize = block.BlockSize()
	return d
}

func (d *DesCrypt) Encode(src []byte, dst []byte) (int, error) {
	dataLen := len(src)
	padding := d.blockSize - dataLen%d.blockSize
	padData := bytes.Repeat([]byte{byte(padding)}, padding) // 明文补码
	src = append(src, padData...)
	dataLen = dataLen + padding
	if dataLen%d.blockSize != 0 {
		return 0, errors.New("need a multiple of the block size")
	}
	offset := 0
	for len(src) > 0 {
		d.block.Encrypt(dst[offset:], src[:d.blockSize])
		src = src[d.blockSize:]
		offset += d.blockSize
	}
	return offset, nil
}

func (d *DesCrypt) Decode(src []byte, dst []byte) (int, error) {
	dataLen := len(src)
	if dataLen%d.blockSize != 0 {
		return 0, errors.New("crypto/cipher: input not full blocks")
	}
	offset := 0
	for len(src) > 0 {
		d.block.Decrypt(dst[offset:], src[:d.blockSize])
		src = src[d.blockSize:]
		offset += d.blockSize
	}
	unPadding := int(dst[offset-1]) // 明文减码
	return offset - unPadding, nil
}

type RsaCrypt struct {
	privateKeyBytes  []byte
	publicKeyBytes   []byte
	privateKey       *rsa.PrivateKey
	publicKey        *rsa.PublicKey
	privateKeyPrefix string
	publicKeyPrefix  string
	privateKeyFile   string
	publicKeyFile    string
}

// RSA 加解密 没有密钥文件时 自动创建
func NewRsaCrypt() *RsaCrypt {
	privateKeyFile := "rsa_private.pem"
	publicKeyFile := "ras_public.pem"

	r := &RsaCrypt{privateKeyPrefix: "RSA PRIVATE KEY", publicKeyPrefix: "RSA PUBLIC KEY", privateKeyFile: privateKeyFile, publicKeyFile: publicKeyFile}
	_, err := os.Stat(publicKeyFile)
	if os.IsNotExist(err) { // 不存在则生成
		if r.CreateRsaKey() != nil {
			return nil
		}
	}
	if file, e0 := os.Open(r.privateKeyFile); e0 != nil {
		return nil
	} else {
		defer file.Close()
		if info, e1 := file.Stat(); e1 != nil {
			return nil
		} else {
			r.privateKeyBytes = make([]byte, info.Size())
			if _, e2 := file.Read(r.privateKeyBytes); e2 != nil {
				return nil
			}
		}
	}
	if file, e0 := os.Open(r.publicKeyFile); e0 != nil {
		return nil
	} else {
		defer file.Close()
		if info, e1 := file.Stat(); e1 != nil {
			return nil
		} else {
			r.publicKeyBytes = make([]byte, info.Size())
			if _, e2 := file.Read(r.publicKeyBytes); e2 != nil {
				return nil
			}
		}
	}

	// public key
	block, _ := pem.Decode(r.publicKeyBytes)
	if block == nil {
		return nil
	}
	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil
	}
	r.publicKey = publicKeyInterface.(*rsa.PublicKey)

	// private key
	block, _ = pem.Decode(r.privateKeyBytes)
	if block == nil {
		return nil
	}
	r.privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil
	}
	return r
}
func (r *RsaCrypt) CreateRsaKey() error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	x509PrivateKey := x509.MarshalPKCS1PrivateKey(privateKey)
	privateFile, err := os.Create(r.privateKeyFile)
	if err != nil {
		return err
	}
	defer privateFile.Close()
	privateBlock := pem.Block{
		Type:  r.privateKeyPrefix,
		Bytes: x509PrivateKey,
	}

	if err = pem.Encode(privateFile, &privateBlock); err != nil {
		return err
	}
	publicKey := privateKey.PublicKey
	x509PublicKey, err := x509.MarshalPKIXPublicKey(&publicKey)
	if err != nil {
		return err
	}
	publicFile, _ := os.Create(r.publicKeyFile)
	defer publicFile.Close()
	publicBlock := pem.Block{
		Type:  r.publicKeyPrefix,
		Bytes: x509PublicKey,
	}
	if err = pem.Encode(publicFile, &publicBlock); err != nil {
		return err
	}
	return nil
}

// 增加了复制 效率较低
func (r *RsaCrypt) Encode(src []byte, dst []byte) (int, error) {
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, r.publicKey, src)
	if err != nil {
		return 0, err
	}
	copy(dst, cipherText)
	return len(cipherText), nil
}

func (r *RsaCrypt) Decode(src []byte, dst []byte) (int, error) {
	plainText, err := rsa.DecryptPKCS1v15(rand.Reader, r.privateKey, src)
	if err != nil {
		return 0, err
	}
	copy(dst, plainText)
	return len(plainText), nil
}

// AES加密
func AesEncrypt(plantText, key string) (string, error) {
	defer func() {
		recover()
	}()
	origData := []byte(plantText)
	data := []byte(key)
	k := len(data)
	keyLen := 16
	if k <= 16 {
		keyLen = 16
	} else if k <= 16 {
		keyLen = 24
	} else {
		keyLen = 32
	}
	keyByte := make([]byte, keyLen)
	copy(keyByte, data)
	block, err := aes.NewCipher(keyByte)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()

	padding := blockSize - len(origData)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	origData = append(origData, padText...)
	blockMode := cipher.NewCBCEncrypter(block, keyByte[:blockSize])
	crypt := make([]byte, len(origData))
	blockMode.CryptBlocks(crypt, origData)
	return string(crypt), nil
}

// AES解密
func AesDecrypt(cipherText, key string) (string, error) {
	defer func() {
		recover()
	}()
	crypt := []byte(cipherText)
	data := []byte(key)
	k := len(data)
	keyLen := 16
	if k <= 16 {
		keyLen = 16
	} else if k <= 16 {
		keyLen = 24
	} else {
		keyLen = 32
	}
	keyByte := make([]byte, keyLen)
	copy(keyByte, data)
	block, err := aes.NewCipher(keyByte)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, keyByte[:blockSize])
	origData := make([]byte, len(crypt))
	blockMode.CryptBlocks(origData, crypt)

	length := len(origData)
	unPadding := int(origData[length-1])
	origData = origData[:(length - unPadding)]
	return string(origData), nil
}
