package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/jordan-wright/email"
	_ "github.com/lib/pq"
)

var config Config

func sendForm(w http.ResponseWriter, r *http.Request) {
	msg := "邮件发送成功"

	if r.Method == "POST" {

		r.ParseForm()

		if refererURL, err := url.Parse(r.Referer()); err == nil {

			//对参数排序
			keys := []string{}
			for k := range r.Form {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			formMap := map[string]string{}
			for _, k := range keys {
				formMap[k] = strings.Replace(r.FormValue(k), `\`, `\`, -1)
			}
			formMap["From Address"] = refererURL.String()

			// 组合模版，并渲染
			body := bytes.NewBuffer([]byte{})
			bodyTmpl := template.New("BodyTemplate")
			bodyTmpl.Parse("{{range $i, $v := .}}{{$i}}:{{$v}}<br/>{{end}}")
			tmplErr := bodyTmpl.Execute(body, formMap)

			//验证电子邮件是否合法
			reply := r.FormValue("_reply_to")
			emailPattern := `^\w+([-+.']\w+)*@\w+([-.]\w+)*\.\w+([-.]\w+)*$`
			matchEmail, _ := regexp.MatchString(emailPattern, reply)

			//获取name字段组合到邮件标题中,并且使用HTML模版渲染进行过滤
			name := strings.Replace(r.FormValue("name"), `\`, `\`, -1)
			subject := bytes.NewBuffer([]byte{})
			subjectTmpl := template.New("SubjectTemplate")
			subjectTmpl.Parse("{{.name}}在{{.host}}有新的留言")
			tmplErr = subjectTmpl.Execute(subject, map[string]string{"name": name, "host": refererURL.Host})

			if matchEmail && tmplErr == nil {
				//发送电子邮件
				if err := sendMail(reply, subject.String(), "", body.String(), config.ToAddress); err != nil {
					msg = err.Error()
				}
			} else {
				msg = "电子邮件地址格式错误或者HTML模版渲染错误"
			}

		} else {
			msg = "来源地址格式错误"
		}

	} else {
		//这里渲染一个静态网页，用于作为主页
		msg = "Get请求"
	}

	pageTmpl := template.New("pageTemplate")
	pageTmpl.Parse("完成:{{.}}")
	_ = pageTmpl.Execute(w, msg)

}

func test(w http.ResponseWriter, r *http.Request) {
	html := `
    <html>
        <body>
            <p>
                <form action="//localhost:3000/" method="POST">
                    name:<input type="text" name="name"><br/>
                    say:<input type="text" name="say"><br/>
                    reply<input type="email" name="_reply_to"><br/>
                    <input type="submit" value="Send">
                </form>
            </p>
        </body>
    </html>
    `
	fmt.Fprintf(w, html)
}

func main() {
	if b, err := ioutil.ReadFile("config.json"); err == nil {
		if err = json.Unmarshal(b, &config); err != nil {
			log.Fatalln("读取配置出错", err.Error())
		}

	}
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {})
	http.HandleFunc("/test", test)             //设置访问的路由
	http.HandleFunc("/"+config.Path, sendForm) //设置访问的路由

	err := http.ListenAndServe(":"+config.Port, nil) //设置监听的端口
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func sendMail(reply string, subject string, text string, html string, to string) (err error) {
	e := email.NewEmail()
	e.From = fmt.Sprintf("%s<%s>", config.EmailName, config.Account)
	e.To = []string{to}
	e.Subject = subject
	e.Text = []byte(text)
	e.HTML = []byte(html)
	e.Headers.Set("Reply-to", reply)
	return e.Send(fmt.Sprintf("%s:%s", config.SMTPHost, config.SMTPPort), smtp.PlainAuth("", config.Account, config.Password, config.SMTPHost))

}

//Config 配置文件对象
type Config struct {
	Port      string `json:"Port"`
	SMTPHost  string `json:"SMTPHost"`
	SMTPPort  string `json:"SMTPPort"`
	Account   string `json:"Account"`
	Password  string `json:"Password"`
	EmailName string `json:"EmailName"`
	ToAddress string `json:"ToAddress"`
	Path      string `json:"Path"`
}
