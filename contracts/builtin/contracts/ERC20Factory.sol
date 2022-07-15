// SPDX-License-Identifier: MIT
pragma solidity 0.8.4;

import "./IBCERC20.sol";

contract ERC20Factory {
    mapping(string => IBCERC20) private _created;

    function createERC20(string memory _name, string memory _symbol) internal {
        //        require(address(_created[name]) == address(0), "ERC20: token already exists");
        _created[_name] = new IBCERC20(_name, _symbol);
    }

    // todo: add some restriction according to usage's environment
    function mintCoins(string memory name, address account, uint256 amount, string memory symbol) public returns (bool) {
        // do create a new token when it does not exist
        if(address(_created[name]) == address(0)) {
            createERC20(name, symbol);
        }

        _created[name].mint(account, amount);
        return true;
    }

    function getBalance(string memory name, address account) public view returns (uint256) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        return _created[name].balanceOf(account);
    }

    function getAllowance(string memory name, address owner, address spender) public view returns (uint256) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        return _created[name].allowance(owner, spender);
    }

    function burnCoins(string memory name, address account, uint256 amount) public returns (bool) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].burn(account, amount);
        return true;
    }

    function transferFrom(string memory name, address sender, address recipient, uint256 amount) public returns (bool) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].transferFrom(sender, recipient, amount);
        return true;
    }

    function increaseAllowance(string memory name, address spender) public returns (bool) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].increaseAllowance(spender, 1000);
        return true;
    }

    function decreaseAllowance(string memory name, address spender) public returns (bool) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        _created[name].decreaseAllowance(spender, 1000);
        return true;
    }

    function destroyCoins(string memory name) public returns (bool) {
        if(address(_created[name]) == address(0)) {
            return true;
        }
        require(_created[name].totalSupply() == 0, "ERC20: total supply is not zero");
        delete _created[name];

        return true;
    }

    function totalSupply(string memory name) public view returns (uint256) {
        require(address(_created[name]) != address(0), "ERC20: token does not exist");

        return _created[name].totalSupply();
    }
}