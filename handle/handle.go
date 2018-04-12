package handle

import (
	"net/http"
	"strings"
	"encoding/json"
	"fmt"
	"gylib/common"
	"gylib/common/redispack"
	"gylib/dbfun"
	"gylib/common/datatype"
	"io/ioutil"
	"time"
	"math/rand"
	"strconv"
	"gylib/common/webclient"
	"net/url"
)

type Json_msg struct {
	Status int         `json:"status"`
	Msg    string      `json:"msg"`
	Data   interface{} `json:"data"`
}

type Controller struct {
	w        http.ResponseWriter
	r        *http.Request
	JsonData map[string]interface{}
	Postdata map[string]interface{}
	Formdata url.Values
	Body     []byte
}

var Site_name string

func init()  {
	redis_ct := redispack.Get_redis_pool()
	client := redis_ct.Get()
	defer client.Close()
 	data := common.Getini("conf/app.ini", "token", map[string]string{"appid": "gysdk", "appkey": ""})
	client.Do("SET", "appid", data["appid"])
	client.Do("SET", "appkey", data["appkey"])
	/*初始化路由表*/
	db:=lib.NewQuerybuilder()
	host_data:=db.Query("select name from sl_host group by name")
	for _,v:=range host_data{
		db.Dbinit()
		host_tmp:=db.Tbname("host").Where(fmt.Sprintf("name='%v'",v["name"])).Select()
		host_json, _ := json.Marshal(host_tmp)
		client.Do("SET", "server_host_"+v["name"], host_json)
		db.Dbinit()
		r_data:=db.Tbname("route").Where(fmt.Sprintf("name='%v'",v["name"])).Select()
		for _,val:=range r_data{
			r_json, _ := json.Marshal(&val)
			client.Do("HSET", "url_route",val["url_map"], r_json)
		}
	}
	site := common.Getini("conf/app.ini", "server", map[string]string{"site_name": ""})
	Site_name=site["site_name"]
	//fmt.Println(get_redis_route("jxcapi/login"))

}

func get_redis_route(url_str string)(string,map[string]string){
	redis_ct := redispack.Get_redis_pool()
	client := redis_ct.Get()
	defer client.Close()
	r_db,_:=client.Do("HGET","url_route",url_str)
	if(r_db==nil){
		return "",nil
	}

	r_data:=make(map[string]string)
	json.Unmarshal(r_db.([]byte),&r_data)

	host,_:=client.Do("GET","server_host_"+r_data["name"])
	if(host==nil){
		return "",r_data
	}
	host_data:=make([]map[string]string,0)
	json.Unmarshal(host.([]byte),&host_data)
	server_ct:=0
	if(len(host_data)>1){
		server_ct=rand.Intn(len(host_data))
	}
	server_host:=host_data[server_ct]["host"]
	server_port:=host_data[server_ct]["port"]
	result:=server_host+":"+server_port
	if(server_port!="80") {
		result = server_host + ":" + server_port
	}
	//fmt.Println(server_ct,result,r_data)
	return result,r_data
}

func get_mysql_route(url_str string)(string,map[string]string){
	db := lib.NewQuerybuilder()
	data := db.Tbname("route").Where(fmt.Sprintf("url_map='%v'", url_str)).Find();
	db.Dbinit()
	host_data:=db.Tbname("host").Where(fmt.Sprintf("name='%v'",data["name"])).Select()
	server_ct:=0;
	if(len(host_data)>1){
		server_ct=rand.Intn(len(host_data))
	}
	server_host:=host_data[server_ct]["host"]
	server_port:=host_data[server_ct]["port"]
	result:=server_host+":"+server_port
	if(server_port!="80") {
		result = server_host + ":" + server_port
	}
	return result,data
}

func ApiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("content-type", "application/json")
	get_url := r.URL.RawQuery                 //获取get字符串
	pathInfo := strings.Trim(r.URL.Path, "/") //获取url路由名称
	//路由为空，设置为根目录
	if(pathInfo==""){
		pathInfo="root"
	}
	mycon := new(Controller)
	mycon.JsonData = make(map[string]interface{})
	mycon.Postdata = make(map[string]interface{})
	mycon.Formdata = make(url.Values)
	mycon.r = r
	mycon.w = w
	mycon.struct_init()
	mycon.url_route(pathInfo, get_url)
}

func (this *Controller) struct_init() {

	if this.r.Method == "GET" {
		this.r.ParseForm()
	} else if(this.r.Method=="POST"){
		//fmt.Println(r.Header)
		//fmt.Println(this.r.Header.Get("sign"))
		//fmt.Println(this.r.Header.Get("Content-Type"))
		form_type := this.r.Header.Get("Content-Type")
		if strings.Contains(form_type, "multipart/form-data;") {
			this.r.ParseMultipartForm(32 << 20)
			this.Formdata=this.r.PostForm
			for k, v := range this.r.PostForm {
				this.Postdata[k] = datatype.Type2str(v)
			}
		} else {
			this.r.ParseForm()
			this.Formdata = this.r.PostForm
			for k, v := range this.r.PostForm {
				this.Postdata[k] = datatype.Type2str(v)
			}
		}
	}else {
		var err error
		this.Body, err = ioutil.ReadAll(this.r.Body)
		if (err != nil) {
			fmt.Println(err)
			return
		}
		if(this.r.Method=="JSON") {
			json.Unmarshal(this.Body, this.JsonData)
		}
	}


}

func (this *Controller) url_route(urlpath, get_url string) {
	url_add,data:=get_redis_route(urlpath)
	if (len(data) == 0) {
		this.error_return("路由错误")
		return
	}
	if (data["is_token"] == "1") {
		if (!this.Check_token()) {
			this.error_return("Token错误");
			return;
		}
	}
	if (data["is_appid"] == "1") {
		if (!this.Check_Signatur()) {
			this.error_return("签名错误");
			return;
		}
	}
	if (data["is_gettoken"] == "1") {
		if (!this.Check_Signatur()) {
			this.error_return("获取token签名错误");
			return;
		}
		this.Get_token()
		return;
	}
	result := ""
	url_add+="/"+data["url"]+"/"
	fmt.Println(url_add)
	if (get_url != "") {
		url_add += "?" + get_url
	}
	if (data["method"] == "MUIFILE") {
		this.Multi_upload(data["upload_file"])
		return
	} else if (data["method"] == "SINFILE") {
		this.Single_upload(data["upload_file"])
		return
	} else if (data["method"] == "POST") {
		result = webclient.Web_Form_POST(url_add, this.Formdata)
	}else if (data["method"] == "GET") {
		result = webclient.HttpGet(url_add)
	}else
	{
		result,_=webclient.DoBytesPost(url_add,this.Body)
	}
	//fmt.Println(string (result))
	//fmt.Println(data["method"])
	data2 := []byte(result)
	this.w.Write(data2)

}

func (this *Controller) error_return(msg string) {
	jsonstr := make(map[string]interface{})
	jsonstr["status"] = 101
	jsonstr["msg"] = msg
	b, _ := json.Marshal(&jsonstr)
	this.w.Write(b)
}

//appid有效性验证
func (this *Controller) Check_Signatur() bool {
	flag := false
	redis_ct := redispack.Get_redis_pool()
	client := redis_ct.Get()
	defer client.Close()
	hasok, _ := client.Do("EXISTS", "appid")
	var appid, appkey, sing string
	if (hasok == 0) {
		data := common.Getini("conf/app.ini", "token", map[string]string{"appid": "gysdk", "appkey": ""})
		appid = data["appid"]
		appkey = data["appkey"]
		client.Do("SET", "appid", data["appid"])
		client.Do("SET", "appkey", data["appkey"])
	} else {
		tmp, _ := client.Do("GET", "appid")
		appid = datatype.Type2str(tmp)
		tmp, _ = client.Do("GET", "appkey")
		appkey = common.Type2str(tmp)
	}
	get_data := make(map[string]string)
	for k, v := range this.r.Form {
		if (k == "sign") {
			sing = datatype.Type2str(v)
			continue
		}
		if (k == "appid") {
			continue
		}
		get_data[k] = datatype.Type2str(v)
	}
	sin_str := common.Signature_MD5(appid, appkey, get_data)
	if (sin_str == sing) {
		flag = true
	}
	return flag
}

func (this *Controller) Check_token() bool {
	token := this.r.FormValue("token")
	if token == "" {
		return false
	} else {
		client := redispack.Get_redis_pool().Get()
		defer client.Close()
		hasok, _ := client.Do("EXISTS", token)
		if hasok == 0 {
			return false
		}
		raw, _ := client.Do("Get", token)
		if raw != nil {
			client.Do("SETEX", token, 3600, raw)
		} else {
			return false
		}

	}
	return true
}

func (this *Controller) Multi_upload(uploadfile string) {
	jsonstr := new(Json_msg)
	jsonstr.Status = 101
	jsonstr.Msg = "上传失败"
	file_list := common.Upload_File(this.r, uploadfile)
	if (len(file_list) > 0) {
		temp_arr := make([]map[string]interface{}, 0)
		for _, val := range file_list {
			if (val != "") {
				temp := make(map[string]interface{}, 0)
				r := rand.New(rand.NewSource(time.Now().UnixNano()))
				temp["title"] = strconv.FormatInt(time.Now().Unix(), 10) + strconv.Itoa(r.Intn(9999))
				temp["img"] = Site_name+val
				temp_arr = append(temp_arr, temp)
			}
		}
		jsonstr.Status = 100
		jsonstr.Data = temp_arr
	}
	this.w.Header().Set("content-type", "application/json")
	b, _ := json.Marshal(&jsonstr)
	//fmt.Println(jsonstr)
	this.w.Write(b)
}

func (this *Controller) Single_upload(uploadfile string) {
	jsonstr := new(Json_msg)
	jsonstr.Msg = "上传失败，请检查"
	jsonstr.Status = 101
	temp_file := common.Upload_File(this.r, uploadfile)
	if (len(temp_file) > 0) {
		if (temp_file[len(temp_file)-1] != "") {
			jsonstr.Data = Site_name+temp_file[len(temp_file)-1]
			jsonstr.Msg = "上传成功"
			jsonstr.Status = 100
		}
	}
	this.w.Header().Set("content-type", "application/json")
	b, _ := json.Marshal(&jsonstr)
	//fmt.Println(string(b))
	//fmt.Println(jsonstr)
	this.w.Write(b)
}

func (this *Controller) Get_token() {
	uu_id := common.Get_UUID()
	Redis_Pool := redispack.Get_redis_pool()
	client := Redis_Pool.Get()
	defer client.Close()
	jsonstr := new(Json_msg)
	jsonstr.Data = uu_id
	data, _ := json.Marshal(&jsonstr.Data)
	client.Do("SETEX", uu_id, 3600, string(data))
	jsonstr.Status = 100
	jsonstr.Msg = "Token获取成功"
	b, _ := json.Marshal(&jsonstr)
	this.w.Write(b)
}
