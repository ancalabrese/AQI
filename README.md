### AQI
Read data from a [PMSA003 Sensor - AirMonitor HAT](https://github.com/sbcshop/Air-Monitoring-HAT/wiki/Wiki).  

This is a golang rewrite of https://github.com/sbcshop/Air-Monitoring-HAT/blob/main/read_example.py
### Why? 
I'm not a Python expert, and it took me an unreasonable amount of time to try running the example script.   
I had to learn about Python virtual environments, install dependencies in venv, install the dependencies for the venv dependencies on my machine... and still that wasn't enough.  

If you are Python ignorant like me, hopefully this will be more straightforward: 
- clone the project on your RPI
- go build aqi.go
- run it
