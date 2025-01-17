Array.prototype.select = function (key) {
    var newList = [];
    for (var i = 0; i < this.length; i++) {
        var obj = {};
        obj[key] = this[i][key];
        newList.insert(i,obj);
    }
    return newList;
};
Array.prototype.insert = function (index, item) {
    this.splice(index, 1, item);
};

Array.prototype.clear = function () {
    this.length = 0;
};

Array.prototype.removeAt = function (index) {
    this.splice(index, 1);
};

Array.prototype.empty = function (query) {
    if (typeof (query) === "function") {
        return this.where(query).length === 0;
    }
    return this.length === 0;
};

Array.prototype.any = function (query) {
    if (typeof(query) === "function") {
        return this.where(query).length > 0;
    }
    return this.length > 0;
};

Array.prototype.removeFirst = function (item) {
    for (var i = 0; i < this.length; i++) {
        if (this[i] == item) {
            this.splice(i, 1);
            return item;
        }
    }
};

Array.prototype.removeAll = function (item) {
    for (var i = 0; i < this.length; i++) {
        if (this[i] == item) {
            this.splice(i, 1);
        }
    }
};

Array.prototype.equals = function (array) {
    var a1 = JSON.stringify(this);
    var a2 = JSON.stringify(array);

    return a1 === a2;
}

Array.prototype.seekAndDestroy = function (w) {
    var founds = [];
    for (var i = 0; i < this.length; i++) {
        if (typeof(w) === "function" && w(this[i])) {
            this.removeAt(i);
            i--;
        }else if(this[i]===w){
            this.removeAt(i);
            i--;
        }
    }
};

Array.prototype.where = function (w) {
    var founds = [];
    for (var i = 0; i < this.length; i++) {
        if (w(this[i])) {
            founds.push(this[i]);
        }
    }
    return founds;
};


Array.prototype.each = function (callback) {
    for (var i = 0; i < this.length; i++) {
        var r = callback(this[i], i);
        if (r == false) break;
    }
};

Array.prototype.map = function (callback) {
    var map = [];
    for (var i = 0; i < this.length; i++) {
        var r = callback(this[i], i);
        map.push(r);
    }
    return map;
};

Array.prototype.contains = function (array) {
    if (!array) return false;
    if (this.length == 0) return false;
    var temp = [];
    var a = array;
    var b = this;
    if (array.length > this.length) {
        a = this;
        b = array;
    }

    for (var i = 0; i < a.length; i++) {
        for (var j = 0; j < b.length; j++) {
            if (b[j] == a[i]) {
                temp.push(b[j]);
                break;
            }
        }
    }
    return a.equals(temp);

};

Array.prototype.last = function () {
    if (this.length > 0) {
        return this[this.length - 1];
    }
    return null;
};

Array.prototype.update = function (array, callback) {
    var i = 0;
    var j = 0;
    for (i = 0; i < array.length; i++) {
        for (j = 0; j < this.length; j++) {
            if (callback(this[j], array[i])) {
                break;
            }
        }
        this[j] = array[i];
    }
}
Array.prototype.innerFor = function (innerArray, callback) {
    for (var i = 0; i < this.length; i++) {
        for (var j = 0; j < innerArray.length; j++) {
            if (callback(this[i], innerArray[j])) {
                return;
            }
        }
    }
}


Array.prototype.sum = function (callback) {
    var acc = 0;
    for (var i = 0; i < this.length; i++) {
        acc += callback(this[i]);
    }
    return acc;
};

Array.prototype.first = function (callback) {
    if (typeof(callback) === "function") {
        for (var i = 0; i < this.length; i++) {
            if (callback(this[i])) {
                return this[i];
            }
        }
    } else {
        if (this.length > 0) {
            return this[0];
        }
        return null;
    }
};


Array.prototype.findIndex = function (callback) {
    if (typeof (callback) === "function") {
        for (var i = 0; i < this.length; i++) {
            if (callback(this[i])) {
                return i;
            }
        }
    }
};

Array.prototype.flatMap = function () {
    var result = [];
    for (var i = 0; i < this.length; i++) {
        if (Array.isArray(this[i])) {
            for(var j=0;j<this[i].length;j++){
              result.push(this[i][j]);   
            }
        }else{
            result.push(this[i]);
        }
    }
    return result;
};
Array.prototype.toDictionary = function (key) {
    var result = {};
    for (var i = 0; i < this.length; i++) {
        if (typeof(key) === "function") {
            result[key(this[i])] = this[i];
        } else {
            result[this[i][key]] = this[i];
        }
    }
    return result;
};
Array.prototype.groupBy = function (key) {
    if(typeof(key) === "function" || typeof(key) === "string"){
        var array = [];
        for (var i = 0; i < this.length; i++) {
            var position, keyValue;
            if(typeof(key) === "string"){
                if(this[i][key] === undefined) throw "Invalid key arg";
                keyValue = this[i][key];
            }else{
                keyValue = key(this[i]);
            }
            position = array.findIndex(function (x) { return x.Key === keyValue });
            if (position !== undefined) {
                array[position].Values.push(this[i]);
            } else {
                array.push({ Key: keyValue, Values: [this[i]] });
            }
        }
        return array;
    }
};

Array.prototype.distinct = function (key) {
    var result = this.toDictionary(key);
    var list = [];
    for(var prop in result){
        if(result.hasOwnProperty(prop)){
            list.push(result[prop]);
        }
    }
    return list;
};


Array.prototype.asyncEach = function (callback,done) {
    var current = 0;
    var arr = this;
    function next(){
        current++;
        if(current < arr.length){
           typeof callback === "function" && callback(arr[current],next,stop);
        }else{
           typeof done === "function" && done();
        }
    }
    var stoped = false;
    function stop(status, error){        
        stoped = true;
    }
    if(stoped) return;
    
    if(arr.length === 0) {
        typeof done === "function" && done();
        return;
    }
    
    typeof callback === "function" && callback(this[current],next,stop);
};



Array.prototype.findOne = function (k,v) {
    for (var i = 0; i < this.length; i++) {
        if (this[i][k] == v) {
            return this[i]
        }
    }
    return undefined;
};