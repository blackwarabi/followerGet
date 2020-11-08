package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"net/smtp"
	"net/url"
	"os"
	"strconv"

	"github.com/ChimeraCoder/anaconda"
	"github.com/bitly/go-simplejson"
)

//出力フォルダパス
const outFolderPath string = "./outFile/"

//フォロワーのUSERIDを出力するファイル名
const oldFollowerFile string = "old.txt"

//処理結果ファイル名
const rsFile string = "result.txt"

/*
メイン処理
*/
func main() {

	var scanner *bufio.Scanner = bufio.NewScanner(os.Stdin)
	fmt.Println("**********処理開始**********")
	fmt.Println("アカウントを番号で指定してください（1：gon_gonk 2：blackwarabi）: ")

	//入力値を取得
	scanner.Scan()
	selectAcc := scanner.Text()

	//選択した番号に対応したアカウント名を変数に格納
	account := ""
	if selectAcc == "1" {
		account = "gon_gonk"
	} else if selectAcc == "2" {
		account = "blackwarabi"
	} else {
		fmt.Println("指定外の番号または不正な値が入力されました。")
		fmt.Println("**********処理完了**********")
		fmt.Scanf("h")
		os.Exit(0)
	}

	//前回のフォロワーのUSERIDを取得する処理
	rsList, e := readOldFollower(outFolderPath, oldFollowerFile, account)
	if err := e; err != nil {
		log.Fatal(err)
	}

	//フォロワー比較処理
	if err := followersComparison(rsList, outFolderPath, rsFile, account); err != nil {
		log.Fatal(err)
	}

	//最新のフォロワー情報を保存する処理
	if err := outputFollower(outFolderPath, oldFollowerFile, account); err != nil {
		log.Fatal(err)
	}

	fmt.Println("**********処理完了**********")
	fmt.Scanf("h")
}

/*
TwitterAPIを呼び出すのに必要なトークン等を設定する
*/
func setTwKey(account string) *anaconda.TwitterApi {
	//TwitterのAPIトークン
	//jsonファイルの読み込み
	bytes, err := ioutil.ReadFile("./context.json")
	if err != nil {
		log.Fatal(err)
	}

	// []byte型からjson型へ変換
	json, _ := simplejson.NewJson(bytes)

	//アカウントで使用トークンを分ける
	var twikey = map[string]string{}
	if account == "blackwarabi" {
		twikey = map[string]string{
			"cons_key":  json.Get("blackwarabi_cons_key").MustString(),
			"cons_sec":  json.Get("blackwarabi_cons_sec").MustString(),
			"accto_key": json.Get("blackwarabi_accto_key").MustString(),
			"accto_sec": json.Get("blackwarabi_accto_sec").MustString(),
		}
	} else if account == "gon_gonk" {
		twikey = map[string]string{
			"cons_key":  json.Get("gonk_cons_key").MustString(),
			"cons_sec":  json.Get("gonk_cons_sec").MustString(),
			"accto_key": json.Get("gonk_accto_key").MustString(),
			"accto_sec": json.Get("gonk_accto_sec").MustString(),
		}
	}

	anaconda.SetConsumerKey(twikey["cons_key"])
	anaconda.SetConsumerSecret(twikey["cons_sec"])
	api := anaconda.NewTwitterApi(twikey["accto_key"], twikey["accto_sec"])
	return api
}

/*
old.txtより前回実行時のフォロワーのUSERIDを取得し、スライスに格納して呼び出し元に返す
*/
func readOldFollower(filepath string, filename string, account string) (reList []string, err error) {
	//outFileフォルダが存在しているかチェックし、なければ新規作成する
	if _, err := os.Stat(outFolderPath); os.IsNotExist(err) {
		os.Mkdir(outFolderPath, 0777)
	}

	//txtファイルより前回のフォロワー一覧を取得する（存在しなければ新規でファイルを作る）
	file, err := os.OpenFile(filepath+account+"_"+filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(file)

	//戻り値用のスライスを宣言
	var rsSlice []string

	//読み取ったold.txtのUSERIDをスライスに格納
	for scanner.Scan() {
		scText := scanner.Text()
		rsSlice = append(rsSlice, scText)
	}
	defer file.Close()
	return rsSlice, nil
}

/*
過去のフォロワーのUSERIDをold.txtファイルから取得し、現在のフォロワーのUSERIDと比較する。
その後、差分が出たUSERIDをresult.txtに出力する
*/
func followersComparison(list []string, filepath string, filename string, account string) error {
	api := setTwKey(account)
	pages := api.GetFollowersIdsAll(nil)
	var rsSlice []string
	for page := range pages {
		for j := 0; len(page.Ids) > j; j++ {
			toString := strconv.Itoa(int(page.Ids[j]))
			rsSlice = append(rsSlice, toString)
		}
	}

	//処理結果用txtファイル作成
	file, err := os.Create(filepath + account + "_" + filename)
	if err != nil {
		log.Fatal(err)
	}

	//比較
	//双方の要素数が前回取得<=今回取得の場合は処理しない
	if len(list) <= len(rsSlice) {
		fmt.Println("処理対象なし")
		return nil
	}

	for i := 0; len(list) > i; i++ {
		//リムーブされたフォロワーを検索し、USERIDをファイルに出力
		if !arrayContains(rsSlice, list[i]) {
			toInt, _ := strconv.Atoi(list[i])
			v := url.Values{}

			//USERIDからスクリーンネームとアカウント名を取得
			userdata, _ := api.GetUsersShowById(int64(toInt), v)
			_, err := file.WriteString("ID:" + userdata.ScreenName + " アカウント名:" + userdata.Name + "\n")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("ID:" + userdata.ScreenName + " アカウント名:" + userdata.Name)
		}
	}

	//処理結果をGMAILで送信
	if err := sendGmail(account); err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	return nil
}

/*
現在のフォロワーのUSERIDを取得し、結果をold.txtに出力する
*/
func outputFollower(filepath string, filename string, account string) error {
	file, err := os.Create(filepath + account + "_" + filename)
	if err != nil {
		log.Fatal(err)
	}
	api := setTwKey(account)
	pages := api.GetFollowersIdsAll(nil)
	for page := range pages {
		for cnt := 0; len(page.Ids) > cnt; cnt++ {
			toString := strconv.Itoa(int(page.Ids[cnt]))
			_, err := file.WriteString(toString + "\n")
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	defer file.Close()
	return nil
}

/*
配列の中に特定のUSERIDが含まれるか探索
*/
func arrayContains(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

/*
result.txtを元にメール送信
*/
func sendGmail(account string) error {
	//jsonファイルの読み込み
	bytes, err := ioutil.ReadFile("./context.json")
	if err != nil {
		log.Fatal(err)
	}

	// []byte型からjson型へ変換
	json, _ := simplejson.NewJson(bytes)
	auth := smtp.PlainAuth(
		"",
		json.Get("address").MustString(),
		json.Get("passwd").MustString(),
		"smtp.gmail.com",
	)

	//txtファイルより処理結果一覧を取得する
	file, err := os.OpenFile(outFolderPath+account+"_"+rsFile, os.O_RDONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(file)
	//メール本文用の文字列を作成
	var message string
	//読み取った内容を連結文字列にする
	for scanner.Scan() {
		message += scanner.Text() + "\r\n"
	}

	err2 := smtp.SendMail(
		"smtp.gmail.com:587",
		auth,
		json.Get("address").MustString(),
		[]string{json.Get("address").MustString()},
		[]byte(
			"To:\r\n"+
				"Subject:リムーブアカウント\r\n"+
				"\r\n"+
				message),
	)
	if err2 != nil {
		log.Fatal(err)
	}
	defer file.Close()
	return nil
}
