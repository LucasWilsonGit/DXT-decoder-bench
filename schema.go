package main

// Book is one book message in sample.json. Each level is ["price","qty"],
// both decimal strings, best level first.
type Book struct {
	Type        string      `json:"type"`
	Instrument  string      `json:"instrument"`
	ID          string      `json:"id"`
	Seq         uint64      `json:"seq"`
	CleanSeq    uint64      `json:"cleanSeq"`
	ExchTime    int64       `json:"exchTime"`
	RecvTime    int64       `json:"recvTime"`
	DecodeTime  int64       `json:"decodeTime"`
	State       string      `json:"state"`
	Recovering  bool        `json:"recovering"`
	Tags        uint64      `json:"tags"`
	Bids        [][2]string `json:"bids"`
	Asks        [][2]string `json:"asks"`
	GwSeq       uint64      `json:"gwSeq"`
	PublishTime int64       `json:"publishTime"`
}

type BookFeed []Book
