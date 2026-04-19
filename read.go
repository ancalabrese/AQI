package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/tarm/serial"
)

// PMSReading represents a sensor reading.
type PMSReading struct {
	// PM10CF1 - PM1.0 concentration unit μ g/m3（CF=1，standard particle).
	PM10CF1 uint16
	//PM25CF1 - PM2.5 concentration unit μ g/m3（CF=1，standard particle).
	PM25CF1 uint16
	// PM100CF1 - PM10 concentration unit μ g/m3（CF=1，standard particle).
	PM100CF1 uint16
	// PM10STD - PM1.0 concentration unit μ g/m3（under atmospheric environment).
	PM10STD uint16
	// PM25STD - PM2.5 concentration unit μ g/m3（under atmospheric environment).
	PM25STD uint16
	// PM100STD - PM10 concentration unit μ g/m3（under atmospheric environment).
	PM100STD uint16 //
	// GR03um -  number of particles with diameter beyond 0.3 um in 0.1L of air.
	GR03um uint16
	// GR05um - number of particles with diameter beyond 0.5 um in 0.1L of air.
	GR05um uint16
	// GR10um - number of particles with diameter beyond 1.0 um in 0.1L of air.
	GR10um uint16
	// GR25um - number of particles with diameter beyond 2.5 um in 0.1L of air.
	GR25um uint16
	// GR50um - number of particles with diameter beyond 5.0 um in 0.1L of air.
	GR50um uint16
	// GR100um - number of particles with diameter beyond 10 um in 0.1L of air.
	GR100um uint16
}

// Sensor handles the serial interface.
type Sensor struct {
	port        *serial.Port
	readTimeout time.Duration
}

// Connect opens the serial connection.
func (s *Sensor) Connect(portName string, baudrate int) error {
	s.readTimeout = time.Second * 2
	config := &serial.Config{
		Name:        portName,
		Baud:        baudrate,
		ReadTimeout: s.readTimeout,
	}

	var err error
	s.port, err = serial.OpenPort(config)
	if err != nil {
		return fmt.Errorf("serial connection: %w", err)
	}
	return nil
}

// Disconnect closes the serial connection.
func (s *Sensor) Disconnect() {
	if s.port != nil {
		s.port.Close()
	}
}

// Read fetches and verifies a single frame from the sensor.
func (s *Sensor) Read() (*PMSReading, error) {
	if s.port == nil {
		return nil, errors.New("sensor not connected")
	}

	buf := make([]byte, 32)
	startTime := time.Now()

	for {
		// Single byte header check
		header := make([]byte, 1)
		_, err := s.port.Read(header)
		if err != nil {
			return nil, err
		}

		// 0x42 (B): (BUS) Start byte for this HAT.
		if header[0] == 0x42 {
			_, err = s.port.Read(header)
			if err != nil {
				return nil, err
			}
			// 0x4D (M): (MESSAGE) - Init sequence.
			if header[0] == 0x4D {
				buf[0], buf[1] = 0x42, 0x4D
				// Consume the ramaining buffer.
				_, err = io.ReadFull(s.port, buf[2:])
				if err != nil {
					return nil, err
				}
				break
			}
		}

		if time.Since(startTime) > s.readTimeout {
			return nil, fmt.Errorf("timeout waiting for valid start frame")
		}
	}

	var checkSum uint16
	for i := 0; i < 30; i++ {
		checkSum += uint16(buf[i])
	}
	sentSum := binary.BigEndian.Uint16(buf[30:])

	if checkSum != sentSum {
		return nil, fmt.Errorf("checksum mismatch: calculated %d, received %d", checkSum, sentSum)
	}

	return &PMSReading{
		PM10CF1:  binary.BigEndian.Uint16(buf[4:6]),
		PM25CF1:  binary.BigEndian.Uint16(buf[6:8]),
		PM100CF1: binary.BigEndian.Uint16(buf[8:10]),
		PM10STD:  binary.BigEndian.Uint16(buf[10:12]),
		PM25STD:  binary.BigEndian.Uint16(buf[12:14]),
		PM100STD: binary.BigEndian.Uint16(buf[14:16]),
		GR03um:   binary.BigEndian.Uint16(buf[16:18]),
		GR05um:   binary.BigEndian.Uint16(buf[18:20]),
		GR10um:   binary.BigEndian.Uint16(buf[20:22]),
		GR25um:   binary.BigEndian.Uint16(buf[22:24]),
		GR50um:   binary.BigEndian.Uint16(buf[24:26]),
		GR100um:  binary.BigEndian.Uint16(buf[26:28]),
	}, nil
}

func main() {
	s := &Sensor{}
	err := s.Connect("/dev/ttyS0", 9600)
	if err != nil {
		log.Fatalf("Critical error: %v", err)
	}
	defer s.Disconnect()

	values, err := s.Read()
	if err != nil {
		log.Fatalf("Read failed: %v", err)
	}

	fmt.Printf("PMS 1 value is %d\n", values.PM10CF1)
	fmt.Printf("PMS 2.5 value is %d\n", values.PM25CF1)
	fmt.Printf("PMS 10 value is %d\n", values.PM100CF1)
}
