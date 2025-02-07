package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/amdf/ipk"
)

var ipkBox ipk.IPK

var sp ipk.Speed
var fas *ipk.AnalogDevice
var fds *ipk.BinaryDevice
var fcs *ipk.FreqDevice

var channel1 ipk.PressureOutput // sensorTM Переменная для задания давления ТM в кгс/см² (канал 1)
var channel2 ipk.PressureOutput // sensorTC Переменная для задания давления ТЦ в кгс/см² (канал 2)
var channel3 ipk.PressureOutput // sensorGR Переменная для задания давления GR в кгс/см²

var channel1BU4 ipk.PressureOutput
var channel2BU4 ipk.PressureOutput
var channelN6 *ipk.DAC

var gBU DescriptionBU
var gDeviceChoice = []string{"БУ-3П", "БУ-3ПА", "БУ-3ПВ", "БУ-4"} // +kpd +CH? todo

// OptionsBU варианты подключаемых БУ:
const (
	BU3P = iota
	BU3PA
	BU3PV
	BU4
)

// OptionsBU варианты подключаемых БУ
type OptionsBU int

// DescriptionBU основные значения БУ
type DescriptionBU struct {
	Name            string
	Variant         OptionsBU
	power           bool
	turt            bool
	BandageDiameter uint32 // бандаж использует ф: sp.Init() но только один бандаж! так что либо 1 либо 2...
	PressureLimit   float64
	NumberTeeth     uint32
	ScaleLimit      uint32
	RelayY          int
	RelayRY         int
	RelayU          int
	// признаки бу4
	NumberDUP  int
	NumberDD   int
	VersionBU4 string
}

func initDataBU(variantBU OptionsBU) (err error) {
	gBU.Variant = variantBU
	gBU.Name = gDeviceChoice[variantBU]

	mapupp, err := readParamFromTOML()
	gUPP = mapupp
	refreshDataBU()

	return
}

func refreshDataIPK() (err error) {

	if err = sp.Init(fcs, gBU.NumberTeeth, gBU.BandageDiameter); err != nil {
		// без запуска потока
		fmt.Printf("InitFreqIpkChannel(): %e", err)
		return
	}

	if gBU.Variant == BU4 {
		channel1BU4.Set(0) // если не выставить ошибка 131
		channel2BU4.Set(0)
	} else {
		if channel2.Init(channelN6, ipk.DACAtmosphere, gBU.PressureLimit); err != nil {
			err = errors.New("ошибка инициализации ЦАП 6: " + err.Error())
		}
	}
	return
}

func initIPK() (err error) {

	ipkBox.AnalogDev = new(ipk.AnalogDevice)
	ipkBox.BinDev = new(ipk.BinaryDevice)
	ipkBox.FreqDev = new(ipk.FreqDevice)

	errF, errB, errA := true, true, true
	if !ipkBox.AnalogDev.Open() {
		errA = false
	}

	if !ipkBox.BinDev.Open() {
		errB = false
	}
	if !ipkBox.FreqDev.Open() {
		errF = false
	}

	if !errA || !errB || !errF {
		str := "Ошибка ИПК. Нет связи с модулем:"
		if !errA {
			str += " ФАС,"
		}
		if !errB {
			str += " ФДС,"
		}
		if !errF {
			str += " ФЧС,"
		}
		str = strings.TrimSuffix(str, ",")
		err = errors.New(str)
		return
	}

	fas = ipkBox.AnalogDev
	fds = ipkBox.BinDev
	fcs = ipkBox.FreqDev

	if err = InitFreqIpkChannel(); err != nil {
		err = errors.New("InitFreqIpkChannel(): " + err.Error())
		return
	}

	// открываем ЦАП 5
	channelN5 := new(ipk.DAC)
	if channelN5.Init(fas, ipk.DAC5); err != nil {
		err = errors.New("ошибка инициализации ЦАП 5: " + err.Error())
		return
	}

	// открываем ЦАП 6
	channelN6 = new(ipk.DAC)
	if channelN6.Init(fas, ipk.DAC6); err != nil {
		err = errors.New("ошибка инициализации ЦАП 6: " + err.Error())
		return
	}

	// открываем ЦАП 7
	channelN7 := new(ipk.DAC)
	if channelN7.Init(fas, ipk.DAC7); err != nil {
		err = errors.New("ошибка инициализации ЦАП 7: " + err.Error())
		return
	}

	// открываем ЦАП 8
	channel8 := new(ipk.DAC)
	if channel8.Init(fas, ipk.DAC8); err != nil {
		err = errors.New("ошибка инициализации ЦАП 8: " + err.Error())
		return
	}

	// открываем ЦАП 9
	channel9 := new(ipk.DAC)
	if channel9.Init(fas, ipk.DAC9); err != nil {
		err = errors.New("ошибка инициализации ЦАП 9: " + err.Error())
		return
	}

	if channel1.Init(channelN5, ipk.DACAtmosphere, 10); err != nil { // максимальное давление 10 кгс/см² (= 10 технических атмосфер) соответствует 20 мА
		err = errors.New("ошибка инициализации ЦАП 5: " + err.Error())
		return
	}

	if channel2.Init(channelN6, ipk.DACAtmosphere, gBU.PressureLimit); err != nil {
		err = errors.New("ошибка инициализации ЦАП 6: " + err.Error())
		return
	}

	if channel3.Init(channelN7, ipk.DACAtmosphere, 10); err != nil {
		err = errors.New("ошибка инициализации ЦАП 7: " + err.Error())
		return
	}

	if channel1BU4.Init(channel8, ipk.DACAtmosphere, 10); err != nil { // максимальное давление 10 кгс/см² (= 10 технических атмосфер) соответствует 20 мА
		err = errors.New("ошибка инициализации ЦАП 8: " + err.Error())
		return
	}
	if channel2BU4.Init(channel9, ipk.DACAtmosphere, 10); err != nil {
		err = errors.New("ошибка инициализации ЦАП 9: " + err.Error())
		return
	}

	channel1.Set(0)
	channel2.Set(0)
	channel3.Set(0)
	channel1BU4.Set(0)
	channel2BU4.Set(0)

	return
}

// InitFreqIpkChannel init
func InitFreqIpkChannel() (err error) {

	if err = sp.Init(fcs, gBU.NumberTeeth, gBU.BandageDiameter); err == nil {

		go func() { // начинаем в фоне обновлять данные по скорости
			for {
				fcs.UpdateFreqDataUSB()
				time.Sleep(time.Second / 4)
			}
		}()
	} else {
		fmt.Printf("InitFreqIpkChannel(): %e", err)
	}
	return
}

// Power питание БУ
func (bu *DescriptionBU) Power(on bool) {
	// 1 -- выкл
	if s1, s2, _ := sp.GetOutputSpeed(); (s1 + s2) > 0 {
		sp.SetSpeed(0, 0)
		sp.SetAcceleration(0, 0)
		time.Sleep(2 * time.Second)
	}

	if on {
		fmt.Println("Включение БУ")
	} else {
		fmt.Println("Выключение БУ")
	}

	err := fds.Set50V(6, !on)
	if err != nil {
		fmt.Println("Ошибка установки двоичного выхода БУ!")
	}
	if gBU.Variant == BU4 {
		fds.Set50V(0, !on)
	}
	bu.power = on
}

// Turt режим обслуживания
func (bu DescriptionBU) Turt(on bool) {
	bu.turt = on
	fds.SetTURT(on)
}

// SetServiceMode перейти в режим обслуживания
func (bu *DescriptionBU) SetServiceMode() {

	if s1, s2, _ := sp.GetOutputSpeed(); (s1 + s2) > 0 {
		sp.SetSpeed(0, 0)
		sp.SetAcceleration(0, 0)
		time.Sleep(4 * time.Second)
	}

	bu.Power(false)
	bu.Turt(true)
	time.Sleep(time.Second)
	bu.Power(true)
	time.Sleep(5 * time.Second)
}

// SetOperateMode рабочий режим
func (bu *DescriptionBU) SetOperateMode() {

	bu.Power(false)
	bu.Turt(false)
	gForm.CheckTurt.SetChecked(false)

	time.Sleep(time.Second)
	bu.Power(true)
	time.Sleep(5 * time.Second)
}

func getNameTOML() (s string) {

	switch gBU.Variant {
	case BU3P:
		s = ".\\toml\\bu3p.toml"
	case BU3PA:
		s = ".\\toml\\bu3pa.toml"
	case BU3PV:
		s = ".\\toml\\bu3pv.toml"
	case BU4:
		s = ".\\toml\\bu4.toml"
	}
	return
}
