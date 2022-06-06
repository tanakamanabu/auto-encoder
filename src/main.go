package main

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	FfmpegPath         string
	EncodeCommand      string
	SilenceCommand     string
	InputPath          string
	OutputPath         string
	InputExt           string
	OutputExt          string
	TargetThresholdSec float64
	RemoveIfSuccess    bool
	Overwrite          bool
}

var config Config

func loadConfig() error {
	_, err := toml.DecodeFile("./config.toml", &config)
	return err
}

func getOutputFileName(fileName string) string {
	ext := filepath.Ext(fileName)
	return config.OutputPath + "\\" + fileName[:len(fileName)-len(ext)] + config.OutputExt
}

func isTarget(file fs.FileInfo) bool {
	//ディレクトリは無視する
	if file.IsDir() {
		return false
	}

	//対象の拡張子以外はスキップする
	if filepath.Ext(file.Name()) != config.InputExt {
		return false
	}

	//現在時刻と比べて、最終更新日が新しいものは録画中の可能性があるのでスキップする
	if time.Now().Sub(file.ModTime()).Seconds() < config.TargetThresholdSec {
		return false
	}

	return true
}

func checkSilent(fileName string) bool {
	//無音になっていたらエンコード失敗なのでチェックする
	params := []string{"-i", fileName}
	params = append(params, strings.Split(config.SilenceCommand, " ")...)
	cmd := exec.Command(config.FfmpegPath, params...)
	var so bytes.Buffer
	var se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err != nil {
		fmt.Printf("『%s』の無音チェックに失敗しました\n", fileName)
		fmt.Println(err.Error())
		fmt.Println(fmt.Sprint(err) + ": " + se.String())
		fmt.Println(config.FfmpegPath)
		fmt.Println(params)
		return true
	}
	out := se.String()
	if strings.Index(out, "silencedetect") != -1 && strings.Index(out, "silence_start:") != -1 {
		return true
	}
	return false
}

func runEncode(inputFileName string, outputFileName string) bool {
	//入力ファイル名を引数としてセット
	params := []string{"-i", inputFileName}

	//設定ファイルのコマンドをセット
	params = append(params, strings.Split(config.EncodeCommand, " ")...)

	//出力ファイル名を引数としてセット
	params = append(params, outputFileName)

	//エンコード処理を実行する
	cmd := exec.Command(config.FfmpegPath, params...)
	var so bytes.Buffer
	var se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err != nil {
		fmt.Println("エンコードに失敗しました")
		fmt.Println(err.Error())
		fmt.Println(fmt.Sprint(err) + ": " + se.String())
		fmt.Println(config.FfmpegPath)
		fmt.Println(params)
		return false
	}

	return true
}

func main() {
	err := loadConfig()

	if err != nil {
		fmt.Println("config.tomlが読めません。")
		return
	}

	files, err := ioutil.ReadDir(config.InputPath)
	if err != nil {
		fmt.Println("InputPathを確認してください。\n" + err.Error())
		return
	}

	//入力ディレクトリのすべてのファイルを調べる
	for _, file := range files {
		//ファイルがエンコード対象かどうかチェックする
		if !isTarget(file) {
			continue
		}

		fmt.Printf("\n\n%sをエンコードします\n", file.Name())

		//出力先のファイル名作成
		outputFileName := getOutputFileName(file.Name())

		if config.Overwrite {
			//上書きするためにエンコード前に削除
			err = os.Remove(outputFileName)
			if err != nil && os.IsExist(err) {
				fmt.Println("既存ファイルの削除に失敗しました。:" + err.Error())
				continue
			}
		} else {
			//出力先に既にファイルがあったらメッセージを表示してスキップ
			_, err = os.Stat(outputFileName)
			if !os.IsNotExist(err) {
				fmt.Println("既に存在するのでエンコードをスキップします。")
				continue
			}
		}

		if !runEncode(config.InputPath+"\\"+file.Name(), outputFileName) {
			continue
		}

		//ちゃんとファイルができたか確認する
		_, err = os.Stat(outputFileName)
		if os.IsNotExist(err) {
			fmt.Println("ファイルが出力されていません。")
			continue
		}

		//無音だったらその旨を表示する
		if checkSilent(outputFileName) {
			fmt.Println("変換結果が無音の動画でした")
			continue
		}

		fmt.Println("エンコードに成功しました")

		if config.RemoveIfSuccess {
			err = os.Remove(config.InputPath + "\\" + file.Name())
			if err == nil {
				fmt.Println("削除しました")
			} else {
				fmt.Println("削除に失敗しました")
				fmt.Println(err.Error())
			}
		}
	}
}
