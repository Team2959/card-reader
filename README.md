## Installation
To install the card scanner service on a raspberry pi, the files in the `prod/`directory need to be copied to the raspberry pi at `/home/pi/prod/` and the `scanner.service` needs to be copied to the services folder and enabled:

 `> sudo systemctl enable scanner.service`
 
Additionally a configuration json file needs to be created with the path `/var/database/config.json`. The file needs to be configured with the parameters shown below:

```
{
    "script_id":"111122222333444AAABBBBCCCDDDEEEFFFGGG444455555",
    "hmac_key":"0000011111aaaabbbbccccdddfff=",
    "device_id":"door_scanner",
    "usb_devices":[
        "Sycreader USB Reader"
    ]
}
```

`script_id` and `hmac_key` should be taken from the setup page of the card scanning google sheet. `device_id` is a user created label to indicate which device these scans are coming from. The `usb_devices` field is a case-insensitive array of device names to allow for use as card readers.