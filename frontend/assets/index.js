var videoInfo = {}
var collectList = undefined;

//获取视频分集列表
function getCList(bvid, func) {
    if (bvid == '') {
        return;
    }
    var url = "https://api.bilibili.com/x/web-interface/view?bvid=" + bvid;
    $.getJSON(url, function (data) {

        if (data.code != 0) {
            layer.msg('未找到相关视频', {
                time: 1500, //20s后自动关闭
            });
            return;
        }
        videoInfo = {
            bvid: data.data.bvid,
            aid: data.data.aid,
            title: data.data.title,
            desc: data.data.desc,
            videos: data.data.videos,
            pic: data.data.pic,
            owner_mid: data.data.owner.mid,
            owner_name: data.data.owner.name,
            owner_face: data.data.owner.face,
            pages: data.data.pages,
        }
        // $.each(data.data.pages, function (i, item) {
        //     list.push(item)
        // });
        func(videoInfo);
    });
}

function convertToDuration(seconds) {
    var hours = Math.floor(seconds / 3600);
    var minutes = Math.floor((seconds % 3600) / 60);
    var remainingSeconds = seconds % 60;

    var duration = "";
    if (hours > 0) {
        duration += hours + ":";
    }
    if (minutes < 10) {
        duration += "0";
    }
    duration += minutes + ":";
    if (remainingSeconds < 10) {
        duration += "0";
    }
    duration += remainingSeconds;

    return duration;
}

function getUrlByCid(aid, cid, func) {
    var url = '';
    if (cid == '') {
        return '';
    }
    var url = "https://api.bilibili.com/x/player/playurl?avid=" + aid + "&cid=" + cid + "&qn=0&type=json&platform=html5";
    $.getJSON(url, function (data) {
        if (data.code != 0) {
            return;
        }
        var info = {
            url: data.data.durl[0].url

        }
        func(info);
    });
}

function getCollectList() {
    if (collectList == undefined) {
        collectList = utools.dbStorage.getItem('collectList')
        if (collectList == undefined) {
            collectList = [];
        }
    }
    return collectList;
}

//增加收藏
function addCollect() {
    if (videoInfo.bvid == undefined) {
        return false;
    }
    var info = collectList.findOne('bvid', videoInfo.bvid)
    if (info != undefined) {
        //取消收藏
        collectList.removeFirst(info)
    } else {
        //增加收藏
        collectList.insert(collectList.length, {
            "time": Date.now(),
            "bvid": videoInfo.bvid,
            "aid": videoInfo.aid,
            "title": videoInfo.title,
            "videos": videoInfo.videos,
            "desc": videoInfo.desc,
            "pic": videoInfo.pic,
            "owner_mid": videoInfo.owner_mid,
            "owner_name": videoInfo.owner_name,
            "owner_face": videoInfo.owner_face
        });
    }

    utools.dbStorage.setItem('collectList', collectList)
    return true;
}

//取消收藏
function deleteCollect(bvid) {
    if (bvid == undefined) {
        return false;
    }
    var info = collectList.findOne('bvid', bvid)
    if (info != undefined) {
        //取消收藏
        collectList.removeFirst(info)
    } else {
        return true;
    }
    utools.dbStorage.setItem('collectList', collectList)
    return true;
}


//检查是否已收藏
function checkCollect(bvid) {
    var info = collectList.findOne('bvid', videoInfo.bvid)
    if (info != undefined) {
        return true;
    } else {
        return false;
    }
}

function showCollect() {
    if (videoInfo == {}) {
        return;
    }
    if (checkCollect(videoInfo.bvid)) {
        $('#info-tools-button-collect').css('background', '#fff799 url(assets/Star.svg)')
    } else {
        $('#info-tools-button-collect').css('background', 'url(assets/Star.svg)')
    }


}


function urlToBVID(url) {
    url = url.trim();
    if (url.length == 0) {
        return false;
    }
    var regx = /^BV(\w{10})$/i
    if (regx.test(url)) {
        return url
    }
    var regx2 = /^https:\/\/www\.bilibili\.com\/video\/BV(\w{10})(\/.*)?$/i
    var matchArray = url.match(regx2);
    if (matchArray != null && matchArray.length > 0) {
        return 'BV' + matchArray[1];
    }
    var regx3 = /^av(\d{8})$/i
    var matchArray3 = url.match(regx3);
    if (matchArray3 != null && matchArray3.length > 0) {
        return enc(matchArray3[1])
    }
    var regx4 = /^https:\/\/www\.bilibili\.com\/video\/av(\d{8})(.*)?$/i
    var matchArray4 = url.match(regx4);
    if (matchArray4 != null && matchArray4.length > 0) {
        return enc(matchArray4[1])
    }

}