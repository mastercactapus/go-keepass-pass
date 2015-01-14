package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"gopkg.in/alecthomas/kingpin.v1"
	"io/ioutil"
	"log"
	"strconv"
	"time"
)

type UUID [16]byte

type BinaryBlobRaw struct {
	ID         int64  `xml:"ID,attr"`
	Compressed bool   `xml:"Compressed,attr"`
	Data       []byte `xml:",innerxml"`
}

type BinaryBlob struct {
	ID   int64
	Data []byte
}

type Group struct {
	UUID   UUID
	Name   string
	Notes  string
	IconID int64
	Times  struct {
		CreationTime         time.Time
		LastModificationTime time.Time
		LastAccessTime       time.Time
		ExpiryTime           time.Time
		Expires              bool
		UsageCount           int64
		LocationChanged      time.Time
	}
	IsExpanded              bool
	DefaultAutoTypeSequence string
	// EnableAutoType          bool
	// EnableSearching         bool
	LastTopVisibleEntry UUID
	Entries             []Entry `xml:"Entry"`
	Groups              []Group `xml:"Group"`
}
type Entry struct {
	UUID   UUID
	IconID int64
	Times  struct {
		CreationTime         time.Time
		LastModificationTime time.Time
		LastAccessTime       time.Time
		ExpiryTime           time.Time
		Expires              bool
		UsageCount           int64
		LocationChanged      time.Time
	}
	Attributes []struct {
		Key   string
		Value string
	} `xml:"String"`
	BinaryAttributes []struct {
		Key   string
		Value struct {
			Ref int64 `xml:"Ref,attr"`
		}
	} `xml:"Binary"`
	AutoType struct {
		Enabled                 bool
		DataTransferObfuscation int64
	}
}

type KeepassDB struct {
	Meta struct {
		Generator                  string
		DatabaseName               string
		DatabaseNameChanged        time.Time
		DatabaseDescription        string
		DatabaseDescriptionChanged time.Time
		DefaultUserName            string
		DefaultUserNameChanged     time.Time
		MaintenanceHistoryDays     int64
		Color                      string
		MasterKeyChanged           time.Time
		MasterKeyChangeRec         int64
		MasterKeyChangeForce       int64
		MemoryProtection           struct {
			ProtectTitle    bool
			ProtectUserName bool
			ProtectPassword bool
			ProtectURL      bool
			ProtectNotes    bool
		}
		RecyleBinEnabled           bool
		RecycleBinUUID             UUID
		RecycleBinChanged          time.Time
		EntryTemplatesGroup        UUID
		EntryTemplatesGroupChanged time.Time
		HistoryMaxItems            int64
		HistoryMaxSize             int64
		LastSelectedGroup          UUID
		LastTopVisibleGroup        UUID
		Binaries                   []BinaryBlob `xml:"Binaries>Binary"`
	} `xml:"Meta"`
	Root struct {
		Groups []Group `xml:"Group"`
	} `xml:"Root"`
}

var (
	file = kingpin.Arg("filename", ".xml file to import").Required().File()
)

func (b *BinaryBlob) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw BinaryBlobRaw
	err := d.DecodeElement(&raw, &start)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(raw.Data)))
	if err != nil {
		return err
	}
	gread, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	data, err = ioutil.ReadAll(gread)
	if err != nil {
		return err
	}
	*b = BinaryBlob{raw.ID, data}
	return nil
}

func (u *UUID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var b64 []byte
	err := d.DecodeElement(&b64, &start)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(b64)))
	if err != nil {
		return err
	}
	*u = [16]byte{}
	copy(u[:], data[0:16])
	return nil
}

func main() {
	kingpin.Parse()
	data, err := ioutil.ReadAll(*file)
	if err != nil {
		log.Fatalln(err)
	}
	var db KeepassDB
	err = xml.Unmarshal(data, &db)
	if err != nil {
		log.Fatalln("Failed to process xml: ", err)
	}

	//todo: clean and dump into pass

	for k := range db.Root.Groups {
		dumpGroup("Root>", &db.Root.Groups[k])
	}
}

func dumpGroup(path string, g *Group) {
	path = path + ">" + g.Name
	for _, e := range g.Entries {
		var title string
		for _, attr := range e.Attributes {
			if attr.Key == "Title" {
				title = attr.Value
				log.Println(path+":", attr.Value)
			}
		}
		for _, battr := range e.BinaryAttributes {
			log.Println(path, ":", title, "{bin["+strconv.Itoa(int(battr.Value.Ref))+"]:", battr.Key+"}")
		}
	}
	for k := range g.Groups {
		dumpGroup(path, &g.Groups[k])
	}
}
