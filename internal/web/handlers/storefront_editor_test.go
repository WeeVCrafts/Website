package handlers

import "testing"

func TestStorefrontMoneyKeepsExactPaise(t *testing.T){
	for _,test:=range []struct{input string;want int64}{{"0",0},{"0.01",1},{"99",9900},{"1499.99",149999},{"1500.00",150000}}{got,e:=parseStorefrontMoney(test.input);if e!=nil||got!=test.want{t.Fatalf("parse(%q)=%d %v",test.input,got,e)}}
	for _,input:=range []string{"-1","1.001","1e3","NaN","","100000000000"}{if _,e:=parseStorefrontMoney(input);e==nil{t.Fatalf("accepted %q",input)}}
}
