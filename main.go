package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"gopkg.in/alecthomas/kingpin.v1"
	"io/ioutil"
	"log"
	"os/exec"
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

type AttrMapRaw []struct {
	Key   string
	Value string
}
type AttrMap map[string]string

type BinMapRaw []struct {
	Key   string
	Value struct {
		Ref int64 `xml:"Ref,attr"`
	}
}
type BinMap map[string]int64

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
	Attributes       AttrMap `xml:"String"`
	BinaryAttributes BinMap  `xml:"Binary"`
	AutoType         struct {
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
	file     = kingpin.Arg("filename", ".xml file to import").Required().File()
	topLevel = kingpin.Flag("top-level", "Create top-level directory").Short('t').Default("false").Bool()
)

func (b *BinMap) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw BinMapRaw
	err := d.DecodeElement(&raw, &start)
	if err != nil {
		return err
	}
	attrMap := make(BinMap, len(raw))
	for _, v := range raw {
		attrMap[v.Key] = v.Value.Ref
	}
	*b = attrMap
	return nil
}

func (b *AttrMap) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var raw AttrMapRaw
	err := d.DecodeElement(&raw, &start)
	if err != nil {
		return err
	}
	if *b == nil {
		*b = make(AttrMap, len(raw))
	}
	for _, v := range raw {
		(*b)[v.Key] = v.Value
	}
	return nil
}

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
	if raw.Compressed {
		gread, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		data, err = ioutil.ReadAll(gread)
		if err != nil {
			return err
		}
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
		var path string
		if *topLevel {
			path = db.Root.Groups[k].Name
		}
		err := dumpGroup(path, &db.Root.Groups[k], &db)
		if err != nil {
			log.Fatalln("Failed while dumping to pass: ", err)
		}
	}
}

func savePw(path string, data []byte) error {
	log.Println("Save: " + path)
	cmd := exec.Command("pass", "insert", "-m", path)
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	_, err = in.Write(data)
	if err != nil {
		return err
	}
	in.Close()
	dat, _ := cmd.CombinedOutput()
	log.Println("Out: " + string(dat))

	return nil
}

func dumpGroup(path string, g *Group, db *KeepassDB) error {
	for _, e := range g.Entries {
		if e.Attributes["Title"] == "" {
			continue
		}
		entryPath := path + "/" + e.Attributes["Title"]
		var pw string
		pw = e.Attributes["Password"] + "\n"
		for name, val := range e.Attributes {
			if name == "Password" || name == "Title" {
				continue
			}
			if val != "" {
				pw = pw + name + ": " + val + "\n"
			}
		}
		err := savePw(entryPath, []byte(pw))
		if err != nil {
			return err
		}

		for name, ref := range e.BinaryAttributes {
			binPath := entryPath + "/" + name
			data := db.Meta.Binaries[ref].Data
			err := savePw(binPath, data)
			if err != nil {
				return err
			}
		}
	}
	for k := range g.Groups {
		err := dumpGroup(path+"/"+g.Groups[k].Name, &g.Groups[k], db)
		if err != nil {
			return err
		}
	}
	return nil
}
