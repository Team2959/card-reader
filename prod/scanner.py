import evdev
import requests
import sqlite3, datetime, uuid, selectors, queue, threading, json, os, hmac, base64

DATABASE_NAME = "/var/database/robotarians.db"
PARAMETER_FILE = "/var/database/config.json"

global SCRIPT_ID, HMAC_KEY, DEVICE_ID, ALLOWED_DEVICES
SCRIPT_ID = ""
HMAC_KEY = ""
DEVICE_ID = ""
ALLOWED_DEVICES = []

def setup():
    conn = sqlite3.connect(DATABASE_NAME)
    c = conn.cursor()
    c.execute('''
        CREATE TABLE IF NOT EXISTS scans (
            card_number TEXT,
            time_stamp TEXT,
            scan_id TEXT,
            UNIQUE(scan_id)
        )
    ''')
    conn.commit()
    conn.close()

    with open(PARAMETER_FILE, "r") as param_file:
        global SCRIPT_ID, HMAC_KEY, DEVICE_ID, ALLOWED_DEVICES
        params = json.loads(param_file.read())
        SCRIPT_ID = params["script_id"]
        HMAC_KEY = params["hmac_key"]
        DEVICE_ID = params["device_id"]
        ALLOWED_DEVICES = [d.lower() for d in params["usb_devices"]]

def send_scans(scans):
    global SCRIPT_ID, HMAC_KEY
    url = "https://script.google.com/macros/s/" + SCRIPT_ID + "/exec"
    try:
        scan_string = json.dumps(scans)
        key = base64.b64decode(HMAC_KEY.encode("ascii"))
        signature = hmac.digest(key, scan_string, "sha384")
        sigenc = base64.b64encode(signature).decode("ascii")

        r = requests.post(url, params={"signature":sigenc}, data=scan_string, headers={"Content-Type":"application/json"})
        if r.status_code != 200 or r.text != "success":
            return False
    except:
        return False
    
    return True

def handle_scans(scan_queue):
    while True:
        scans = []
        try:
            # Wait for data for 30.0 seconds, timeout only in place to avoid deadlock
            card, timestamp, scan_id = scan_queue.get(block=True, timeout=30.0)
            scans.append({
                "scan_id":scan_id,
                "timestamp":timestamp,
                "card_number":card,
                "device_id":DEVICE_ID
            })
        except queue.Empty:
            # Timeout has expired
            continue
        except:
            # This is probably not recoverable
            exit()
        
        # Add scan to database
        conn = sqlite3.connect(DATABASE_NAME)
        c = conn.cursor()

        if not send_scans(scans):
            # if scans are not send successfully store them in the local database
            c.executemany("INSERT INTO scans VALUES (?,?,?)", [(s["card_number"], s["timestamp"], s["scan_id"]) for s in scans])
        else:
            # if scans are send successfully fetch all scans from the local database and send them as well
            for row in c.execute("SELECT card_number, time_stamp, scan_id FROM scans"):
                scans = []
                scans.append({
                    "scan_id":row[2],
                    "timestamp":row[1],
                    "card_number":row[0],
                    "device_id":DEVICE_ID
                })

            if send_scans(scans):
                # if stored scans successfully send remove them from the database
                c.executemany("DELETE FROM scans WHERE scan_id = ?", [(s["scan_id"],) for s in scans])

        conn.commit()
        conn.close()


def get_scanners():
    global ALLOWED_DEVICES
    all_devices = [evdev.InputDevice(path) for path in evdev.list_devices()]
    usb_readers = [dev for dev in all_devices if dev.name.lower() in ALLOWED_DEVICES]
    return usb_readers

if __name__ == "__main__":
    setup()

    scanners = get_scanners()

    # it is important that multiple scanner results aren't intermixed
    selector = selectors.DefaultSelector()

    for dev in scanners:
        selector.register(dev, selectors.EVENT_READ)
        dev.grab()

    scanned_strings = {}
    scan_queue = queue.SimpleQueue()

    scan_thread = threading.Thread(target=handle_scans, args=(scan_queue,))
    scan_thread.start()

    try:
        while True:
            for key, mask in selector.select():
                device = key.fileobj
                for event in device.read():
                    if event.type == evdev.ecodes.EV_KEY:
                        key_event = evdev.categorize(event)
                        if key_event.keystate:
                            code = str(key_event.keycode)
                            if code.startswith("KEY_"):
                                code = code[len("KEY_"):]
                            if not code.isnumeric():
                                # the scanned string is now complete
                                # handle the code
                                # pass the current string into a handle, then clear it
                                scan_id = str(uuid.uuid4())
                                time_stamp = datetime.datetime.now(datetime.timezone.utc).isoformat()
                                card_num = scanned_strings[device.path]

                                # camera_queue.put(scan_id)
                                scan_queue.put((card_num, time_stamp, scan_id))
                                
                                scanned_strings[device.path] = ""
                            else:
                                if not device.path in scanned_strings:
                                    scanned_strings[device.path] = ""
                                scanned_strings[device.path] += code

    finally:
        for dev in scanners:
            dev.ungrab()
