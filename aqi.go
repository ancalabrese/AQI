package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/tarm/serial"
)

// ReadingResult is the result of a reading.
// If Err is not nill, discard any data in Data.
type ReadingResult struct {
	Data PMSReading
	Err  error
}

// PMSReading represents a sensor reading.
type PMSReading struct {
	// T is the time of the reading.
	Time time.Time
	// D is the reading data
	Data PMData
}

// PMData is the data for a reading.
type PMData struct {
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

// Read fetches and verifies data from the sensor.
func (s *Sensor) Read(ctx context.Context, n int, interval time.Duration) <-chan ReadingResult {
	rr := make(chan ReadingResult)

	go func() {
		defer close(rr)

		for range n {
			select {
			case <-ctx.Done():
				return
			default:
				reading, err := s.takeReading()
				rr <- ReadingResult{
					Data: reading,
					Err:  err,
				}
				time.Sleep(interval)
			}
		}
	}()
	return rr
}

func (s *Sensor) takeReading() (PMSReading, error) {
	if s.port == nil {
		return PMSReading{}, errors.New("sensor not connected")
	}

	buf := make([]byte, 32)
	startTime := time.Now()
	// Single byte header check
	header := make([]byte, 1)
	_, err := s.port.Read(header)
	if err != nil {
		return PMSReading{}, fmt.Errorf("reading header byte error: %w", err)
	}

	var readTime time.Time
	for {
		// 0x42 (B - BUS): Start byte.
		if header[0] == 0x42 {
			_, err = s.port.Read(header)
			if err != nil {
				return PMSReading{}, fmt.Errorf("reading init sequence byte error: %w", err)
			}
			// 0x4D (M - MESSAGE): Init sequence.
			if header[0] == 0x4D {
				buf[0], buf[1] = 0x42, 0x4D
				// Consume the ramaining buffer.
				_, err = io.ReadFull(s.port, buf[2:])
				if err != nil {
					return PMSReading{}, fmt.Errorf("reading buffer err: %w", err)
				}
				readTime = time.Now()
				break // got the data, exit lpop for processing.
			}
		}

		if time.Since(startTime) > s.readTimeout {
			return PMSReading{}, errors.New("timeout waiting for valid start frame")
		}
	}

	var checkSum uint16
	for i := range 30 {
		checkSum += uint16(buf[i])
	}
	sentSum := binary.BigEndian.Uint16(buf[30:])

	if checkSum != sentSum {
		return PMSReading{}, fmt.Errorf("checksum mismatch: calculated %d, received %d", checkSum, sentSum)
	}

	return PMSReading{
		Time: readTime,
		Data: PMData{
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
		},
	}, nil
}

var (
	nReads   int
	interval time.Duration
)

func init() {
	flag.IntVar(&nReads, "n", 1, "Number of consecutive sensor reads")
	flag.DurationVar(&interval, "i", 5*time.Second, "Interval in between sensor reads (i.e. 1s, 2m, 500ms")
}

func main() {
	flag.Parse()
	ctx := context.Background()
	s := &Sensor{}
	err := s.Connect("/dev/ttyS0", 9600)
	if err != nil {
		log.Fatalf("Critical error: %v", err)
	}
	defer s.Disconnect()

	r := s.Read(ctx, nReads, interval)

	for val := range r {
		if val.Err != nil {
			fmt.Printf("Error: reading failed %s\n", err.Error())
			continue
		}

		s := &strings.Builder{}
		fmt.Fprintf(s, "Time: %v\n", val.Data.Time)
		fmt.Fprintf(s, "\tPMS 1 value is %d\n", val.Data.Data.PM10CF1)
		fmt.Fprintf(s, "\tPMS 2.5 value is %d\n", val.Data.Data.PM25CF1)
		fmt.Fprintf(s, "\tPMS 10 value is %d\n", val.Data.Data.PM100CF1)
		fmt.Println(s.String())
	}
}
