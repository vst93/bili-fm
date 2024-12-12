
var ticket = null;
var qrcodeKey = null;
var loginStatus = false;

console.log('preload.js 已加载');

// async function searchVideo(keyword, order){
//   // BLLogin()
//   switch (order) {
//     case 'click':
//       order = 'click';
//       break;
//     case 'update':
//       order = 'pubdate';
//       break;
//     default:
//       order = 'totalrank';
//   }
//   const list = await getBiliTicket('').then(() => {
//     var options = {
//       'method': 'GET',
//       'url': 'https://api.bilibili.com/x/web-interface/wbi/search/type?search_type=video&page=1&page_size=50&order=' + order + '&keyword=' + encodeURIComponent(keyword),
//       'headers': {
//         'authority': 'api.bilibili.com',
//         'accept': '*/*',
//         'accept-language': 'zh-CN,zh;q=0.9',
//         'origin': 'https://search.bilibili.com',
//         'referer': 'https://search.bilibili.com/video',
//         'user-agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36 Edg/116.0.1938.62',
//         'Cookie': 'bili_ticket=' + ticket
//       }
//     };
//     return new Promise((resolve, reject) => { 
//       return window.go.main.BL.Request(options, function (error, response) {
//         if (error) throw new Error(error);
//         const json = JSON.parse(response.body);
//         const data = [];
//         json.data.result.forEach(item => {
//           item.title = item.title.replace(/<em class="keyword">/g, '').replace(/<\/em>/g, '');
//           data.push({
//             picture_url: `https:${item.pic}`,
//             url: `https://www.bilibili.com/video/${item.bvid}`,
//             title: item.title,
//             views: item.play > 1000000? `${(item.play / 10000).toFixed(1)}万` : item.play,
//             danmuCount: item.video_review,
//             author: item.author,
//             date: timestamp2Date(item.pubdate), // 发布日期,时间戳转日期格式 YYYY-MM-DD
//           });
//         });
//         resolve(data)
//       })
//     })
//   });
//   return list;
// }


// async function getBiliTicket(csrf) {
//   if (ticket) {
//     return ticket;
//   }
//   const ts = Math.floor(Date.now() / 1000);
//   const hexSign = window.go.main.BL.HmacSha256('XgwSnGZ1p', `ts${ts}`);
//   const url = 'https://api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket';
//   const params = new URLSearchParams({
//     key_id: 'ec02',
//     hexsign: hexSign,
//     'context[ts]': ts,
//     csrf: csrf || ''
//   });

//   const options = {
//     method: 'POST',
//     url: `${url}?${params.toString()}`,
//     headers: {
//       'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/115.0'
//     }
//   };

//   try {
//     window.go.main.BL.Request(options, function (error, response, body) {
//       if (error) {
//         throw error;
//       }
//       if (response.statusCode !== 200) {
//         throw new Error(`HTTP error! status: ${response.statusCode}`);
//       }
//       const data = JSON.parse(body);
//       if (data.code !== 0) {
//         return null;
//       }
//       ticket = data.data.ticket;
//       return data.data.ticket;
//     });
//   } catch (e) {
//     throw e;
//   }
// }


// async function getLoginQRCode () {
//   console.log('获取登录二维码');
//   var options = {
//     'method': 'GET',
//     'url': 'https://passport.bilibili.com/x/passport-login/web/qrcode/generate',
//     'headers': {
//       'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36'
//     }
//   };
//   return new Promise((resolve, reject) => {
//     return window.go.main.BL.Request(options, function (error, response) {
//       let data = {};
//       if (error) throw new Error(error);
//       let json = JSON.parse(response.body);
//       if (json.code === 0) {
//         qrcodeKey = json.data.qrcode_key;
//         data = json.data.url
//       }
//       resolve(data);
//     });
//   })
  
// }

async function getLoginQRCodeStatus() {
  if (!loginStatus) { 
    return false;
  }
  console.log('获取登录二维码状态' + qrcodeKey);
  var options = {
    'method': 'GET',
    'url': 'https://passport.bilibili.com/x/passport-login/web/qrcode/poll?qrcode_key=' + qrcodeKey,
    'headers': {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36'
    }
  };
  return new Promise((resolve, reject) => {
    return window.go.main.BL.Request(options, function (error, response) {
      let data = {};
      if (error) throw new Error(error);
      let json = JSON.parse(response.body);
      // console.log(json);
      if (json.code === 0) {
        if (json.data.code === 0) {
          // console.log('二维码扫描成功');
          let cookie = response.headers['set-cookie'];
          cookie = cookie.join(';');
          console.log(cookie);
          // utools.dbStorage.setItem('SESSDATA', cookie)
          localStorage.setItem('SESSDATA', cookie)
          resolve(true);
        } else if (json.data.code === 86038) {
          console.log('二维码已失效');
          getLoginQRCode();
          resolve(false);
        } else {
          // console.log('二维码扫描中...');
          setTimeout(() => {
            if (loginStatus) {
              resolve(getLoginQRCodeStatus(qrcodeKey));
            }
          }, 2000);
        }
      }
    });
  })
} 

// function getLoginStatus() { 
//   return loginStatus;
// }

// function setLoginStatus(status) { 
//   loginStatus = status;
// }

// function getSESSDATA() {
//   return localStorage.getItem('SESSDATA');
//   // return utools.dbStorage.getItem('SESSDATA');
// }

async function getBLFavFolderList() {
  const cookie = getSESSDATA();
  if (!cookie) {
    // utools.showNotification('请登录后再操作');
    console.log('请登录后再操作');
    return [];
  }
  const url = 'https://api.bilibili.com/x/v3/fav/folder/created/list-all';
  const params = new URLSearchParams({
    // 'up_mid': utools.dbStorage.getItem('mid') || 0,
    'up_mid': localStorage.getItem('mid') || 0,
    'type':2,
  });
  const options = {
    'method': 'GET',
    'url': `${url}` + '?' + params.toString(),
    'headers': {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36',
      'Cookie': cookie,
    }
  };
  return new Promise((resolve, reject) => {
    return window.go.main.BL.Request(options, function (error, response) {
      let data = [];
      if (error) throw new Error(error);
      let json = JSON.parse(response.body);
      // console.log(json);
      if (json.code === 0) {
        data = json.data.list;
      }
      resolve(data);
    });
  })
}


async function getBLUserInfo() { 
  const cookie = getSESSDATA();
  if (!cookie) { 
    return null;
  }
  const url = 'https://api.bilibili.com/x/web-interface/nav';
  const options = {
    'method': 'GET',
    'url': `${url}`,
    'headers': {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36',
      'Cookie': cookie,
    }
  };
  return new Promise((resolve, reject) => {
    return window.go.main.BL.Request(options, function (error, response) {
      let data = {};
      if (error) throw new Error(error);
      let json = JSON.parse(response.body);
      // console.log(json);
      if (json.code === 0) {
        data = {
          uname: json.data.uname,
          face: json.data.face,
          mid: json.data.mid
        };
        // utools.dbStorage.setItem('uname', data.uname)
        // utools.dbStorage.setItem('face', data.face)
        // utools.dbStorage.setItem('mid', data.mid)

        localStorage.setItem('uname', data.uname)
        localStorage.setItem('face', data.face)
        localStorage.setItem('mid', data.mid)
      }
      resolve(data);
    });
  })
}

async function getBLFavFolderListDetail(fid,page = 1) { 
  const cookie = getSESSDATA();
  if (!cookie) {
    // utools.showNotification('请登录后再操作');
    console.log('请登录后再操作');
    return [];
  }
  const url = 'https://api.bilibili.com/x/v3/fav/resource/list';
  // console.log(fid,page)
  const params = new URLSearchParams({
    'media_id': fid,
    'type': 0,
    'ps': 21,
    'pn': page,
  });
  const options = {
    'method': 'GET',
    'url': `${url}` + '?' + params.toString(),
    'headers': {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36',
      'Cookie': cookie,
    }
  };
  return new Promise((resolve, reject) => {
    return window.go.main.BL.Request(options, function (error, response) {
      let data = [];
      if (error) throw new Error(error);
      let json = JSON.parse(response.body);
      // console.log(json);
      if (json.code === 0) {
        data = json.data.medias;
      }
      resolve(data);
    });
  })
}

async function getBLFeedList(offset = 0) {
  const cookie = getSESSDATA();
  if (!cookie) {
    // utools.showNotification('请登录后再操作');
    console.log('请登录后再操作');
    return {
      items: [],
      has_more: false,
      offset: 0,
    };
  }
  const url = 'https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/all';
  // console.log(offset)
  let theParams = {
    'type': 'video',
  };
  if (offset) { 
    theParams.offset = offset;
  }
  const params = new URLSearchParams(theParams);
  const options = {
    'method': 'GET',
    'url': `${url}` + '?' + params.toString(),
    'headers': {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36',
      'Cookie': cookie,
    }
  };
  return new Promise((resolve, reject) => {
    return window.go.main.BL.Request(options, function (error, response) {
      let data = {
        items: [],  
        has_more: false,
        offset: 0,
      };
      if (error) throw new Error(error);
      let json = JSON.parse(response.body);
      // console.log(json);
      if (json.code === 0) {
        data.items = json.data.items;
        data.has_more = json.data.has_more;
        data.offset = json.data.offset;
      }
      resolve(data);
    });
  })
}


async function getBLRCMDList(page = 1) {
  const cookie = getSESSDATA();
  if (!cookie) {
    // utools.showNotification('请登录后再操作');
    console.log('请登录后再操作');
    return {
      items: [],
      has_more: false,
      page: page,
    };
  }
  const url = 'https://api.bilibili.com/x/web-interface/wbi/index/top/feed/rcmd';
  let theParams = {
  };
  const params = new URLSearchParams(theParams);
  const options = {
    'method': 'GET',
    'url': `${url}` + '?' + params.toString(),
    'headers': {
      'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36',
      'Cookie': cookie,
    }
  };
  return new Promise((resolve, reject) => {
    return window.go.main.BL.Request(options, function (error, response) {
      let data = {
        items: [],
        has_more: false,
        page: page,
      };
      if (error) throw new Error(error);
      let json = JSON.parse(response.body);
      // console.log(json);
      if (json.code === 0) {
        if (page < 10) {
          data.has_more = true;
        } else { 
          data.has_more = false;
        }
        data.items = json.data.item;
        data.page = page;
      }
      resolve(data);
    });
  })
}


async function BLLogin() {
  let qrcodeUrl = await getLoginQRCode();
  console.log(qrcodeUrl);
  $('#bl-loginqrcode').attr('src', 'https://api.pwmqr.com/qrcode/create/?url=' + encodeURIComponent(qrcodeUrl));
  while (true) {
    if (!getLoginStatus()) {
      console.log('break2');
      break;
    }
    let status = await getLoginQRCodeStatus();
    if (status) {
      console.log('break');
      $('.login-panel').hide();
      refreshUserInfo();
      break;
    }
  }
}