// Authors: Sergio Roldan

$(document).ready(function () {

    // Perfect scrollbars
    const ps4 = new PerfectScrollbar('.list-group.peers');
    const ps5 = new PerfectScrollbar('.list-group.users');
    const ps6 = new PerfectScrollbar('.messageBox');

    $("#privButton").attr("disabled", true);

    // Interval for private messages
    var refreshIntervalId;

    // Private modal open and interval set
    $(".users").on("dblclick", ".list-group-item", function(e) {
        name = $(e.target).text()

        $(".chatBox").animate({opacity: 1}, 1000);
        $("#privButton").attr("disabled", false);

        $("#peerName").text(name)
        getPrivateMessages(name)

        refreshIntervalId = setInterval(() => {
            getPrivateMessages(name)
        }, 1000);
    })

    // Private message submit
    $("#privateMsgForm").submit(function(e) {

        // Prevent default, get value, clean value/errors, check is not empty and send ajax
        e.preventDefault();

        var actionurl = e.currentTarget.action;
        var input = $("#privateMsgForm #privateMsgTextArea").val();
        $("#privateMsgForm #privateMsgTextArea").val("");
        var identity = $("#privateMsgForm #privateMsgHash").val();

        $("#msgForm .toast").each(function() {
            $(this).remove();
        });

        if (input != "") {
            $.ajax({
                url: '/message',
                type: 'post',
                data: { text: input, destination: $("#peerName").text(), identity: identity },
            });
        } else {
            $("<div class=\"toast\" role=\"alert\" aria-live=\"assertive\" aria-atomic=\"true\"><div class=\"toast-header\"><strong class=\"mr-auto\">Error Message</strong><button type=\"button\" class=\"ml-2 mb-1 close\" data-dismiss=\"toast\" aria-label=\"Close\"><span aria-hidden=\"true\">&times;</span></button></div><div class=\"toast-body\">The message must be non empty</div></div>").appendTo("#peersterForm");
        }
    });

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
            data: peer
        });
    });

    function getPrivateMessages(user) {
        $.ajax({
            url: window.location.href + "signal",
            type: 'post',
            data: { name: user },
            success: function(data) {
                var array = JSON.parse(data);
                $('.messageBox').html("")
                array.forEach(function(priv) {
                    let newmsg = $("<div class=\"card message\"><div class=\"card-body\"><h5 class=\"card-title\">" + priv.Origin + "</h5><h5 class=\"card-subtitle mb-2 text-muted\">" + priv.ID + "</h5><p class=\"card-text\">" + priv.Text + "</p></div></div>").appendTo(".messageBox")
                    if (priv.Origin != user)
                        newmsg.addClass("mine")

                    $(".messageBox").scrollTop($(".messageBox")[0].scrollHeight);
                })
            }
        });
    }

    // get gossiper name and modify title
    $.ajax({
        url: window.location.href + "id",
        type: 'get',
        success: function(data) {
            $("#peerID").text(data)
        }
    });

    // get gossiper identity
    $.ajax({
        url: window.location.href + "identity",
        type: 'get',
        success: function(data) {
            tmpdata = data.replace(/\$/g, "\n<b>").replace(/"/g, "").replace(/%/g, "</b>")
            $("#identity").html(tmpdata)
        }
    });

    // do all the functions periodically (every second, see last parameter to change), in order to update the lists in the the user interface with the latest values
    window.setInterval(function () {
        
        // update peers list
        function updateNodeBox() {
            $.get("/node", function (data) {
                var array = JSON.parse(data);

                array.forEach(function(peer) {
                    if ($(".list-group.peers .list-group-item").text().indexOf(peer) == -1) {
                        $("<li class=\"list-group-item\">" + peer + "</li>").prependTo(".list-group.peers")
                        $(".list-group.peers").scrollTop($(".list-group.peers")[0].scrollHeight);
                    }
                });
            });
        }
        updateNodeBox()

        // update origin list
        function updateOriginBox() {
            $.get("/origin", function (data) {
                var array = JSON.parse(data);

                array.forEach(function(peer) {
                    if ($(".list-group.users .list-group-item").text().indexOf(peer) == -1) {
                        $("<li class=\"list-group-item\">" + peer + "</li>").prependTo(".list-group.users")
                        $(".list-group.users").scrollTop($(".list-group.users")[0].scrollHeight);
                    }
                });
            });
        }
        updateOriginBox()

    }, 1000);
});