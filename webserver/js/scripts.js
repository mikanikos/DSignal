$(document).ready(function () {

    // send message action: send post message with specified paramters 
    $("#sendForm").submit(function (event) {

        event.preventDefault();

        var message = document.getElementById("messageInput").value;

        if (message == "") {
            return
        }

        document.getElementById("messageInput").value = "";

        $.ajax({
            url: '/message',
            type: 'post',
            data: { text: message, destination: "" },
        });
    });

    // add peer action: send post message with peer address
    $("#peerForm").submit(function (event) {

        event.preventDefault();

        var peer = document.getElementById("peerInput").value;
        document.getElementById("peerInput").value = "";

        if (peer == "") {
            return
        }

        $.ajax({
            url: '/node',
            type: 'post',
            data: peer,
        });
        updatePeersList()

    });

    // send private message with double click on origin
    $('#originList').dblclick(function (e) {
        var origin = e.target.textContent;
        var message = prompt("Enter the message to send to " + origin);
        if (message != null && message != "") {
            $.ajax({
                url: '/message',
                type: 'post',
                data: { text: message, destination: origin },
            });
        }
    });

    // download file with double click on search result
    $('#searchList').dblclick(function (e) {
        var fileToDownload = e.target.textContent;
        var result = fileToDownload.split(" ");
        $.ajax({
            url: '/message',
            type: 'post',
            data: { file: result[0], request: result[1] },
        });

    });

    // display prompt to download a file with parameters
    $('#buttonDownloadFile').click(function (e) {
        var requestHex = prompt("Enter the hexadecimal metahash of the file to download");
        if (requestHex != null && requestHex != "") {
            var dest = prompt("Enter the name of the peer who has the file");
            if (dest != null && dest != "") {
                var fileName = prompt("Enter the name of the file");
                if (fileName != null && fileName != "") {

                    $.ajax({
                        url: '/message',
                        type: 'post',
                        data: { text: "", destination: dest, file: fileName, request: requestHex },
                    });
                }
            }
        }
    });

    // display prompt to search for a file with parameters
    $('#buttonSearchFile').click(function (e) {
        var keywordsValue = prompt("Enter the keywords of the file you wanna search on the other nodes");
        if (keywordsValue != null && keywordsValue != "") {
            var budgetValue = prompt("Enter the initial budget for the search (leave empty for default 2 with doubling each second)");
            if (budgetValue == null || budgetValue == "") {
                budgetValue = "0"
            }

            $.ajax({
                url: '/message',
                type: 'post',
                data: { text: "", keywords: keywordsValue, budget: budgetValue },
            });
        }
    });

    // hide file selected description of the picker
    $("#fileInput").css('opacity', '0');

    // trigger event when file picked
    $("#buttonFile").click(function (e) {
        e.preventDefault();
        $("#fileInput").trigger('click');
    });

    // send file indexed with post message
    $("#fileInput").change(function () {
        var input = $(this).val().split(/(\\|\/)/g).pop()
        $.ajax({
            url: '/message',
            type: 'post',
            data: { text: "", destination: "", file: input },
        });
    });

    // get gossiper name and modify title
    $.get("/id", function (data) {
        data = data.replace("\"", "").replace("\"", "")
        document.getElementById("peerID").innerHTML = "DSignal - ID: " + data;
    });

    // do all the functions periodically (every second, see last parameter to change), in order to update the lists in the the user interface with the latest values
    window.setInterval(function () {
        
        // update blockchain
        function updateBlockchainBox() {
            $.get("/blockchain", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('blockchainList');

                while (list.hasChildNodes()) {
                    list.removeChild(list.firstChild)
                }

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el["Name"] + " " + el["MetaHash"]
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateBlockchainBox()

        // update round number
        function updateRound() {
            $.get("/round", function (data) {
                data = data.replace("\"", "").replace("\"", "")
                document.getElementById('round').innerHTML = "Round: " + data;
            });
        }
        updateRound()
        
        // update peers list
        function updateNodeBox() {
            $.get("/node", function (data) {
                var array = JSON.parse(data);

                var list = document.getElementById('peerList');
                while (list.hasChildNodes()) {
                    list.removeChild(list.firstChild)
                }

                for (el of array) {
                    var entry = document.createElement('li');
                    entry.appendChild(document.createTextNode(el));
                    list.appendChild(entry);
                }
            });
        }
        updateNodeBox()

        // update origin list
        function updateOriginBox() {
            $.get("/origin", function (data) {
                var array = JSON.parse(data);

                var list = document.getElementById('originList');
                while (list.hasChildNodes()) {
                    list.removeChild(list.firstChild)
                }

                for (el of array) {
                    var entry = document.createElement('li');
                    entry.appendChild(document.createTextNode(el));
                    list.appendChild(entry);
                }
            });
        }
        updateOriginBox()

        // update blochain log messages
        function updateBCLogs() {
            $.get("/bcLogs", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('bcLogsList');

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateBCLogs()

        // update file indexed list
        function updateFileBox() {
            $.get("/file", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('fileList');

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el["Name"] + ", " + el["Size"] + " KB " + el["MetaHash"]
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateFileBox()

        // update file downloaded list
        function updateDownloadBox() {
            $.get("/download", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('downloadList');

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el["Name"] + ", " + el["Size"] + " KB " + el["MetaHash"]
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateDownloadBox()

        // update search results
        function updateSearchBox() {
            $.get("/search", function (data) {
                var jsonData = JSON.parse(data);
                var list = document.getElementById('searchList');

                // while (list.hasChildNodes()) {
                //     list.removeChild(list.firstChild)
                // }

                for (el of jsonData) {
                    var entry = document.createElement('li');
                    var text = el["Name"] + " " + el["MetaHash"]
                    entry.style.margin = "10px"
                    entry.appendChild(document.createTextNode(text));
                    list.appendChild(entry);
                }
            });
        }
        updateSearchBox()

        // get latest messages and add them to list
        $.get("/message", function (data) {
            var jsonData = JSON.parse(data);
            var list = document.getElementById('messageList');

            for (el of jsonData) {
                var entry = document.createElement('li');
                var text = "[" + el["Origin"] + "] " + el["Text"]
                entry.appendChild(document.createTextNode(text));
                list.appendChild(entry);
            }
        });
    }, 1000);
});